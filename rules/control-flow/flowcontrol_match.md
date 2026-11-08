# `match`

- **Status:** Normative
- **Created:** 2026-08-17
- **Last updated:** 2026-11-08 (stable chapter, subsection, and paragraph numbering); 2026-10-28 (§§ 5, 8, 25A, 28 reachable-state exhaustiveness, proven-inactive arms and payload-bearing variant patterns, MD-029/MD-042); 2026-09-03
- **Document revision:** 2.2
- **Sec language version:** 0.1
- **Canonical path:** `rules/control-flow/flowcontrol_match.md`
- **Replaces:** `rules/control-flow/flowcontrol_match.txt`
- **Repository baseline reviewed:** `50655d3bc5d2e2596935dc5609cb1db1387e0cec`
- **Implementation governance:** `governance/control_flow.yaml`

---

## § 1 Purpose

**§ 1(1)** `match` performs exhaustive structural and variant-based branching.

**§ 1(2)** It is used for:

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

**§ 1(3)** `match` is not Sec's general value-comparison construct.

**§ 1(4)** Use:

- `if` for a `bool` condition;
- `switch` for ordinary literal, range, or relational value selection;
- `match` for structural or variant selection.

**§ 1(5)** Sec 0.1 does not define general runtime type matching or a recursive pattern language.

---

## § 2 Core syntax

**§ 2(1)** Statement form:

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

**§ 2(2)** Expression form:

```sec
let message := match status {
    Status.Ready => "ready"
    Status.Waiting => "waiting"
    Status.Failed => "failed"
}
```

**§ 2(3)** Guarded arm:

```sec
match option {
    Some(value) where value > 100 => Large(value)
    Some(value) => Normal(value)
    None => Missing()
}
```

**§ 2(4)** A `match` must contain at least one arm.

---

## § 3 Subject evaluation

**§ 3(1)** The subject expression is evaluated exactly once.

```sec
match ReadResult() {
    Ok(value) => Process(value)
    Err(error) => Handle(error)
}
```

**§ 3(2)** `ReadResult()` executes exactly once.

**§ 3(3)** The resulting subject value or subject Place is the single semantic subject for all arm tests, guards, payload bindings, ownership actions, and final ownership-state merging.

**§ 3(4)** The backend must not re-evaluate the source expression while lowering individual arms.

---

## § 4 Arm ordering

**§ 4(1)** Arms are considered in source order.

**§ 4(2)** For each arm:

1. the pattern is tested;
2. when the pattern matches, candidate pattern bindings become visible to the guard;
3. when a guard exists, the guard is evaluated;
4. when the guard is false, the arm is not selected and matching continues with the next arm;
5. when the guard is true, the arm is selected;
6. ownership actions associated with the selected arm are committed according to this rulebook;
7. the arm body executes.

**§ 4(3)** Exactly one arm executes.

**§ 4(4)** No later pattern or guard is evaluated after an arm has been selected.

---

## § 5 Exhaustiveness

**§ 5(1)** `match` is exhaustive.

**§ 5(2)** Every reachable semantic state of the subject must be covered by the arm set or by an unguarded catch-all pattern.

**§ 5(3)** Coverage is based on the resolved subject type and proven control-flow state, not merely on source spelling.

**§ 5(4)** A guarded arm does not by itself exhaust its underlying pattern because the guard may evaluate to `false`.

**§ 5(5)** A finite set of guarded whole-payload arms may jointly cover a variant when
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

**§ 5(6)** This is exhaustive.

**§ 5(7)** This is not:

```sec
match option {
    Some(value) where value > 0 => Positive(value)
    None => Missing()
}
```

**§ 5(8)** because `Some` values for which the guard is false remain uncovered.

**§ 5(9)** **Reachable-state exhaustiveness (MD-042).** When authoritative analysis soundly
proves that the subject can only be in a smaller variant set at the match
point, only that set needs coverage:

```sec
type Figure union {
    Dot
    Line(int)
    Circle(int)
}

let figure := Figure.Dot

// Exhaustive because the subject is proven to be Figure.Dot here.
match figure {
    Dot => DrawDot()
}
```

**§ 5(10)** The proof must hold for the actual subject: immutable known value facts,
trusted path refinements and applicable compiler-known provenance. Mutation,
reassignment, aliases, foreign or dynamic calls, property reads with unproved
stability, invalidated flow assumptions and unknown results conservatively
widen or invalidate it; the declaration's initial value is never used after it
may have changed. Without a sound proof, exhaustiveness uses the whole domain
of the resolved subject type, including every legal union/enum/`Result`/`Option`
case, a reachable `empty` state and the unknown encodings of open bit-backed
enums (§ 27). Absence of proof is not a license to omit arms. Guard semantics
are unchanged: a guarded arm is not complete coverage merely because its
variant is known. Arms excluded only by such a proof are legal (§ 25A).

---

## § 6 Catch-all pattern

**§ 6(1)** The catch-all pattern is:

```text
_
```

**§ 6(2)** It matches every remaining reachable state not handled by an earlier selected arm.

```sec
match direction {
    Direction.North => North()
    _ => Other()
}
```

**§ 6(3)** An unguarded catch-all must be the final reachable arm.

**§ 6(4)** An arm after an unguarded catch-all is unreachable and is invalid.

**§ 6(5)** A guarded catch-all is not exhaustive by itself:

```sec
match value {
    _ where condition => Handle()
    _ => Fallback()
}
```

---

## § 7 Result errors must not be hidden

**§ 7(1)** A catch-all must not silently hide an unhandled `Err` variant.

**§ 7(2)** This is invalid when `_` would absorb `Err`:

```sec
match result {
    Ok(value) => Process(value)
    _ => Ignore()
}
```

**§ 7(3)** Handle the error explicitly:

```sec
match result {
    Ok(value) => Process(value)
    Err(error) => Handle(error)
}
```

**§ 7(4)** An explicit `Err(_)` pattern handles the error variant and discards its payload
without introducing a binding. This is valid:

```sec
match result {
    Err(_) => Fallback()
    Ok(value) => Process(value)
}
```

**§ 7(5)** This differs from the catch-all `_`, which may not hide an unhandled `Err`.
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

## § 8 Variant patterns

**§ 8(1)** A payload-less variant is matched by its resolved variant name.

```sec
match direction {
    Direction.North => North()
    Direction.South => South()
    _ => Other()
}
```

**§ 8(2)** Namespace qualification follows the normal name-resolution rules.

**§ 8(3)** Variant spelling is resolved semantically. The backend must not infer variant identity from textual names.

**§ 8(4)** A **bare variant pattern** is also valid for a variant that carries a payload
(MD-029). It selects every value of that variant without extracting the
payload; it is not a test that the payload is empty or default:

```sec
match figure {
    Figure.Rectangle => HandleRectangle()
    Figure.Circle => HandleCircle()
}
```

**§ 8(5)** An unguarded bare variant contributes full coverage of that variant; a guarded
one contributes only the guarded subset. Because data exists but is not
inspected, the compiler issues a configurable **Information** diagnostic by
default that names the payload type or fields and suggests a binding form.
`Variant(_)` for a single payload and `Variant {}` for a struct-like payload
state deliberate non-binding and suppress only that information; they never
waive ownership, destructor or non-discardability obligations (§ 15). The same
rule applies to `Err(...)` error variants (`rules/errors/errorhandling.md`
§ 15.3).

---

## § 9 Whole-payload binding

**§ 9(1)** A variant carrying one payload may bind that payload as a whole value.

```sec
match result {
    Ok(value) => Process(value)
    Err(error) => Handle(error)
}
```

**§ 9(2)** A struct-like union variant may also bind its complete payload object when that form is defined for the union:

```sec
match shape {
    Circle(circle) => Process(circle.Radius)
    Rectangle(rectangle) => Process(rectangle.Width, rectangle.Height)
}
```

**§ 9(3)** The binding exists only within that arm's guard and body according to the guard rules below.

---

## § 10 Shallow named-field destructuring

**§ 10(1)** Struct-like union payloads support canonical shallow named-field destructuring.

```sec
match shape {
    Rectangle { width, height } => Use(width, height)
    Circle { radius } => UseRadius(radius)
}
```

**§ 10(2)** Partial binding is valid and requires no `..` marker:

```sec
Rectangle { width } => Use(width)
```

**§ 10(3)** Omitted fields:

- are not bound;
- do not constrain the matched value beyond variant selection;
- are not copied or moved merely because the pattern omits them.

**§ 10(4)** Fields may be renamed:

```sec
Rectangle { width: w, height: h } => Use(w, h)
```

**§ 10(5)** Fields may be borrowed:

```sec
Rectangle { width: ref w, height: ref h } => Inspect(w, h)
Rectangle { width: ref mut w } => Modify(w)
```

**§ 10(6)** Destructuring is shallow.

**§ 10(7)** Sec 0.1 does not define recursive nested destructuring such as:

```sec
Some(Ok(value)) => Use(value)
```

**§ 10(8)** Use another `match` for the nested value.

---

## § 11 The `empty` pattern

**§ 11(1)** A mutable non-defaultable union binding may have the compiler-known initialization state `empty` according to the union rules.

**§ 11(2)** When that state is reachable, `match` may inspect it:

```sec
match state {
    empty => Initialize()
    Idle => Wait()
    Running => Work()
}
```

**§ 11(3)** `empty` is not:

- a union variant;
- a value of the union type;
- `None`;
- `Err`;
- `null`;
- a hidden numeric tag exposed to the program.

**§ 11(4)** If control-flow analysis proves that the subject is initialized, `empty` is not part of exhaustiveness.

**§ 11(5)** If `empty` is reachable, exhaustive coverage must include it or an unguarded catch-all.

**§ 11(6)** `_` covers reachable `empty` state together with every other remaining reachable state.

**§ 11(7)** A moved-from union is not `empty`.

---

## § 12 Payload binding scope

**§ 12(1)** Pattern bindings exist only within their arm.

**§ 12(2)** They are visible:

- in that arm's guard;
- in that arm's body.

**§ 12(3)** They are not visible:

- in another arm;
- after the `match`.

**§ 12(4)** Normal Sec no-shadowing rules apply.

```sec
let value := 10

match result {
    Ok(value) => Process(value)
    Err(error) => Handle(error)
}
```

**§ 12(5)** is invalid because the pattern binding would shadow the existing `value`.

---

## § 13 Whole-payload ownership modes

**§ 13(1)** Pattern bindings follow normal Sec ownership and borrowing rules.

**§ 13(2)** For a whole-payload plain binding:

```sec
Some(value)
```

**§ 13(3)** where the payload type is `T`:

- if `T` is implicitly copyable, the binding receives a copy;
- if `T` is move-only and the subject is an owned reusable Place from which ownership may legally be transferred, the binding receives the payload by move;
- if the subject is a fresh temporary, the payload may be forwarded directly into the binding without creating a reusable moved-from source binding;
- the compiler must never silently clone the payload;
- the compiler must never silently change a by-value binding into a borrow.

**§ 13(4)** For explicit shared borrowing:

```sec
Some(ref value)
```

**§ 13(5)** `value` has type `ref T`.

**§ 13(6)** For explicit mutable borrowing:

```sec
Some(ref mut value)
```

**§ 13(7)** `value` has type `ref mut T`, and the subject must provide mutable borrowing authority.

**§ 13(8)** The borrow is branch-scoped and must satisfy the normal overlap and escape rules.

**§ 13(9)** A fresh temporary subject, such as `match MakeOption() { Some(ref value) => ... }`, may also be borrowed from with `ref` or `ref mut`, for its whole payload or a named payload field such as `Boxed { head: ref h }` (owner decision 2026-11-07). The temporary lives through the whole match, the arm borrows its payload in place, and the borrow ends with the arm, before the temporary is destroyed (see `rules/memory/destruction.md` § 8(8)). Like any payload borrow, it cannot escape the arm. A subject that is itself a reference returned by a call has no temporary to borrow from; its payload origin is that of the returned reference.

**§ 13(10)** A subject constructed in place, such as `match Some(<-handle) { ... }`, takes the operands it is built from when the subject is evaluated, exactly as the same construction does in a `let` initializer. After the match, `handle` is moved.

---

## § 14 Borrowed subjects cannot transfer ownership

**§ 14(1)** A subject accessed only through `ref UnionType` does not own the union value and therefore cannot transfer ownership of a move-only payload.

**§ 14(2)** A plain by-value payload binding from a shared borrowed subject is valid only when the payload can be copied under the normal copy rules.

**§ 14(3)** Move-only payload access requires an explicit borrow:

```sec
Some(ref value)
```

**§ 14(4)** A `ref mut UnionType` subject may provide:

```sec
Some(ref value)
Some(ref mut value)
```

**§ 14(5)** according to ordinary borrowing authority.

**§ 14(6)** It still cannot move an owned payload out merely because mutable access exists. Mutable borrow authority is not ownership authority.

---

## § 15 No hidden partial moves from shallow field destructuring

**§ 15(1)** Shallow named-field destructuring must not create hidden partial moves from a union payload.

**§ 15(2)** A plain field binding is therefore by-value copy only.

```sec
Packet { header, payload }
```

**§ 15(3)** is valid for a field only when that field's type is implicitly copyable.

**§ 15(4)** If a destructured field is move-only, plain by-value field binding is a compile-time error.

**§ 15(5)** Use:

```sec
Packet { payload: ref p }
```

**§ 15(6)** or:

```sec
Packet { payload: ref mut p }
```

**§ 15(7)** when borrowing is intended.

**§ 15(8)** When ownership of move-only content is required, bind the complete payload by value and perform explicit ownership operations on that complete value.

**§ 15(9)** The compiler must not silently clone a field and must not leave the reusable union subject in a hidden partially moved state through shallow field destructuring.

---

## § 16 Guards

**§ 16(1)** A guard uses `where`:

```sec
Pattern where condition => body
```

**§ 16(2)** The guard expression must have type `bool`.

**§ 16(3)** The pattern is tested before the guard.

**§ 16(4)** The guard is evaluated only when the pattern matches.

**§ 16(5)** If the guard evaluates to `false`, matching continues with the next arm.

**§ 16(6)** Bindings introduced by the pattern are visible in the guard.

---

## § 17 Guards and move-only payloads

**§ 17(1)** Ownership transfer for a move-only by-value payload binding must not be committed merely because the pattern matched.

**§ 17(2)** The move is committed only after:

1. the pattern matches; and
2. the guard, when present, evaluates to `true`; and
3. the arm is selected.

**§ 17(3)** Example:

```sec
match option {
    Some(value) where IsUsable(ref value) => Consume(value)
    Some(value) => HandleOther(value)
    None => Missing()
}
```

**§ 17(4)** When `value` is move-only:

- the first pattern may expose a candidate binding to the guard;
- the guard may inspect or borrow the candidate under normal rules;
- the guard must not consume the prospective by-value binding before arm selection;
- if the guard is false, no move from the subject has occurred because of that arm;
- later arms must see the ownership state that exists after the guard's actual side effects but without a committed payload move from the rejected arm;
- if the guard is true, the move into `value` is committed before the arm body uses `value` as its owned binding.

**§ 17(5)** A consuming use of a prospective move-only binding inside the guard is invalid.

**§ 17(6)** Explicit `ref` and `ref mut` pattern bindings create the corresponding candidate borrow for guard evaluation. A borrow from an arm whose guard fails ends before the next arm is tested, subject to ordinary side effects already performed by the guard.

---

## § 18 Ownership state after `match`

**§ 18(1)** Ownership analysis is path-sensitive across match arms.

**§ 18(2)** Every continuing arm contributes its resulting ownership state.

**§ 18(3)** If a move-only payload is moved from a reusable union subject in an arm, the compiler must mark the affected subject or Place unavailable on that path according to the ownership model.

**§ 18(4)** Example:

```sec
match value {
    Some(resource) => Consume(<-resource)
    None => Nothing()
}

Use(value)
```

**§ 18(5)** If `resource` is move-only and the `Some` arm continues after consuming the payload, `value` is not definitely available after the `match`.

**§ 18(6)** The post-match merge must conservatively combine the continuing arm states.

**§ 18(7)** Conceptually:

```text
Available + Moved
    ConditionallyAvailable
```

**§ 18(8)** A later whole-value use is invalid unless every continuing path establishes an available value through legal reinitialization.

**§ 18(9)** A terminating arm does not contribute an ownership state to a later merge point because that path does not continue.

**§ 18(10)** The compiler must enforce this rule. It is not an advisory analysis.

---

## § 19 Statement `match`

**§ 19(1)** A `match` may be used only for control flow and side effects.

```sec
match result {
    Ok(value) => Process(value)
    Err(error) => Handle(error)
}
```

**§ 19(2)** An expression arm in statement context is evaluated in statement context.

**§ 19(3)** A produced value must not disappear merely because the surrounding construct is `match`.

**§ 19(4)** The normal discard rules apply:

- ordinary implicitly discardable call results may use the established implicit call-result discard behavior;
- must-use values may not be implicitly discarded;
- explicit discard must be written when required or intentionally used;
- ownership and destruction of discarded results follow `discard.md`.

**§ 19(5)** When a statement arm needs explicit discard, use a block:

```sec
match option {
    Some(value) => {
        discard Calculate(value)
    }

    None => {}
}
```

**§ 19(6)** `match` itself does not create a new blanket implicit-discard rule.

---

## § 20 Expression `match`

**§ 20(1)** A `match` may produce a value.

```sec
let message := match status {
    Status.Ready => "ready"
    Status.Waiting => "waiting"
    Status.Failed => "failed"
}
```

**§ 20(2)** Every continuing arm must produce a value assignable to one common result type.

**§ 20(3)** A branch that returns, propagates, or otherwise terminates does not need to produce the match result value.

```sec
let value := match result {
    Ok(value) => value
    Err(error) => return Err(error)
}
```

**§ 20(4)** The match result's ownership classification follows its resolved result type and the ownership action that produced each continuing arm result.

---

## § 21 Value-producing arm blocks

**§ 21(1)** Sec 0.1 supports a contextual value-producing block specifically in the result position of an expression-match arm.

```sec
let price := match product {
    Book(book) => {
        LogBook(book)
        CalculateBookPrice(book)
    }

    Service(service) => service.Price
}
```

**§ 21(2)** In such a block, the final expression on every continuing path is the arm result.

**§ 21(3)** `CalculateBookPrice(book)` above is therefore the value produced by the `Book` arm.

**§ 21(4)** This rule does not make arbitrary Sec blocks into expressions.

**§ 21(5)** Outside this contextual match-arm result position, ordinary block semantics remain unchanged.

### § 21.1 Continuing paths

**§ 21.1(1)** Every path that reaches the end of a value-producing arm block must reach a final value expression assignable to the match result type.

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

**§ 21.1(2)** The `return` path terminates. The continuing path produces `Transform(data)`.

**§ 21.1(3)** This is invalid when `Log(data)` returns `void`:

```sec
let result := match value {
    A(data) => {
        Log(data)
    }

    B => DefaultValue()
}
```

**§ 21.1(4)** because the continuing `A` arm does not produce the required result value.

### § 21.2 Ownership of the final expression

**§ 21.2(1)** If the final expression produces or transfers a move-only value, ownership of the match result must be established before cleanup destroys arm-local values required to produce that result.

**§ 21.2(2)** Conceptually, arm-result ownership is committed before arm-scope cleanup runs.

**§ 21.2(3)** The backend may optimize representation but must preserve this semantic ordering.

---

## § 22 Result type compatibility

**§ 22(1)** All continuing value-producing arms must produce compatible types under the normal Sec type system.

**§ 22(2)** Named type identity remains significant.

**§ 22(3)** No match-specific implicit conversion is introduced.

```sec
let value := match option {
    Some(number) => number
    None => "missing"
}
```

**§ 22(4)** is invalid when `number` has type `int`.

**§ 22(5)** Likewise, different named semantic types remain distinct even when their representation types are compatible.

---

## § 23 Return and termination analysis

**§ 23(1)** `match` participates in function return analysis.

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

**§ 23(2)** An exhaustive match whose every arm terminates is itself non-continuing.

**§ 23(3)** If at least one reachable arm continues, control may continue after the match.

---

## § 24 Definite assignment

**§ 24(1)** An exhaustive match can establish definite assignment when every continuing arm establishes the required assignment state.

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

**§ 24(2)** Only continuing arms contribute to the state after the match.

**§ 24(3)** The same path-sensitive rule applies to initialization state, union `empty` state, ownership state, borrow state, and other compiler analyses that merge across control flow.

---

## § 25 Duplicate and unreachable arms

**§ 25(1)** A duplicate unguarded pattern is invalid.

```sec
match option {
    None => A()
    None => B()
    Some(value) => C(value)
}
```

**§ 25(2)** A pattern fully covered by an earlier unguarded arm is unreachable.

```sec
match option {
    Some(value) => A(value)
    Some(value) where value > 10 => B(value)
    None => C()
}
```

**§ 25(3)** The guarded `Some` arm is unreachable because the earlier unguarded `Some` already covers every `Some` value.

**§ 25(4)** A guarded arm may precede an unguarded fallback for the same pattern:

```sec
match option {
    Some(value) where value > 10 => Large(value)
    Some(value) => Other(value)
    None => Missing()
}
```

## § 25A Proven-inactive arms (MD-042)

**§ 25A(1)** A correctly typed arm for a variant that is impossible **solely because of the
subject's proven current state** (§ 5) is **legal**. It may be kept for
clarity, reviewability or a future broader value source. The compiler reports
it with a default **Information** diagnostic that explains the proof, not with
an error:

```sec
match figure {
    Dot => DrawDot()
    Line(length) => DrawLine(length)      // Legal; provably inactive here.
    Circle(radius) => DrawCircle(radius) // Legal; provably inactive here.
}
```

**§ 25A(2)** An inactive arm must still be syntactically valid, name a declared variant,
use a valid binding shape, type-check its guard and body and respect ownership
and lifetime obligations.

**§ 25A(3)** Proven inactivity differs from source-order redundancy. The compiler
distinguishes:

1. a structurally invalid, duplicate or earlier-covered arm — **Error** (this
   section above, `S3003`);
2. a valid arm excluded only by a proven subject state — **Information**;
3. an arm not proven excluded — no inactivity diagnostic.

**§ 25A(4)** The classification does not depend on whether lowering later removes the arm.
The formatter never removes inactive arms.

**§ 25A(5)** The declared error type and the errors a specific expression can produce are
distinct facts. Widening a core error enum does not make every operation able
to produce every new variant: a `try` whose failures provably arise only from
an integer division may keep a legally typed
`Err(ArithmeticError.PrecisionExhausted)` handler as informationally inactive.
A protected subtree with further calls or failure-producing operands must
include every reachable failure source, and a declaration such as
`fn Calculate() Result[int, ArithmeticError]` proves nothing about which
variants its implementation can produce. Failure provenance restricts the
active error set only when it is complete and sound; it is a semantic proof,
not an ABI property or an inference from a variant name.

**§ 25A(6)** Rules: `rules/corrections/applied/md029-md042-md102-pattern-reachability-some-correction-20261028.md` §§ 8–10.

---

## § 26 Ordinary enums

**§ 26(1)** Ordinary enums are closed over their declared underlying value classes,
including integer and string values.

**§ 26(2)** Exhaustiveness is based on distinct reachable declared values, not merely member spellings.

**§ 26(3)** If multiple enum members are aliases for the same underlying value, they
represent one runtime value class for coverage and duplicate/unreachable
analysis.

**§ 26(4)** A match need not contain separate arms for every alias spelling.

**§ 26(5)** The compiler must normalize enum aliases before coverage analysis.

---

## § 27 Open bit-backed enums

**§ 27(1)** A `bit` or `bit[N]` hardware enum is open over the complete representable bit domain.

**§ 27(2)** Declared members do not necessarily exhaust that domain.

**§ 27(3)** Therefore:

```sec
match status {
    Status.Ready => Ready()
    Status.Error => Error()
}
```

**§ 27(4)** is exhaustive only when the declared members provably cover the complete representable bit domain.

**§ 27(5)** Otherwise an additional arm such as:

```sec
_ => UnknownEncoding()
```

**§ 27(6)** is required.

**§ 27(7)** The compiler must base exhaustiveness on the actual bit width and normalized numeric value classes.

---

## § 28 `Result[T, E]`

**§ 28(1)** `Result[T, E]` uses the same match machinery with compiler-known variants:

```sec
match result {
    Ok(value) => Success(value)
    Err(error) => Failure(error)
}
```

**§ 28(2)** General `match` does not propagate `Err` implicitly.

**§ 28(3)** Both success and error states must be covered explicitly according to the Result/error rules.

**§ 28(4)** Use `try` when success should continue implicitly and errors should be propagated or handled through try-handler semantics.

**§ 28(5)** Use `match` when both `Ok` and `Err` belong to explicit branch control flow.

### § 28.1 Concrete error variants

**§ 28.1(1)** An ordinary `match` on `Result[T, ConcreteError]` may use
`Err(ConcreteError.Variant)` to match one specific variant of the closed
concrete error payload:

```sec
match result {
    Ok(value) => Use(value)

    Err(IOError.NotFound) => HandleMissing()
    Err(IOError.AccessDenied) => HandleDenied()
    Err(IOError.Timeout) => HandleTimeout()
}
```

**§ 28.1(2)** Specific `Err(ConcreteError.Variant)` arms participate in ordinary closed
exhaustiveness. When every variant of the concrete error type is covered
together with `Ok`, the match is exhaustive without an `Err(error)` arm. When
one or more variants remain uncovered, the match is non-exhaustive unless
another valid arm covers them.

**§ 28.1(3)** `Err(error)` binds the complete error payload and covers the complete `Err`
branch, so an `Err(ConcreteError.Variant)` arm after it is unreachable, and an
`Err(error)` arm after arms covering every variant is unreachable as well.
Concrete-variant arms before an `Err(error)` arm are valid.

**§ 28.1(4)** The variant must belong to the subject's own error type. Enum error variants
are compared by their underlying value class, like ordinary enum arms. A
payload-carrying union error variant may be selected as a bare variant, with
its single payload bound (`Err(FileError.NotFound(info))`) or with shallow
field destructuring (`Err(FileError.NotFound { Path: path })`); destructuring
is one level deep (MD-029; `rules/errors/errorhandling.md` §§ 15.3, 27.2b).
With one unambiguous closed concrete error type the contextual
`Err(.Variant...)` spelling is also valid (activated in Sec 0.1.2).

**§ 28.1(5)** This is an explicit, deliberately narrow Sec 0.1 Result/error refinement
exception to the restricted nested-pattern model (§ 32). It is not a general
recursive or destructuring pattern facility and not a permanent pattern-model
guarantee; a later Sec version may replace or generalize it. Open-error
narrowing for `Result[T, error]` below is unchanged (MD-008; `rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md` § 9).

---

## § 29 `Option[T]`

**§ 29(1)** `Option[T]` uses compiler-known variants:

```sec
match option {
    Some(value) => Use(value)
    None => Missing()
}
```

**§ 29(2)** `None` is a real `Option[T]` value.

**§ 29(3)** It is not the union initialization state `empty`.

**§ 29(4)** Whole-payload binding of `Some` follows the ownership rules in this document.

### § 29.1 Open `error` narrowing

**§ 29.1(1)** `Result[T, error]` preserves the concrete identity and payload of widened Sec
errors. Its `Err` branch may therefore use error-specific concrete patterns:

```sec
match result {
    Ok(value) => Use(value)
    Err(IOError.NotFound) => UseDefault()
    Err(errorValue) => Handle(errorValue)
}
```

**§ 29.1(2)** The root `error` domain is open. Concrete error arms alone are not exhaustive;
an `Err(errorValue)` or `Err(_)` fallback is required unless control-flow facts
prove a narrower closed state. Generic `_` still may not hide an unhandled
`Err`. This is an error-specific narrowing rule, not general runtime type
matching.

---

## § 30 Direct boolean matching is not part of Sec 0.1

**§ 30(1)** A plain `bool` subject should use `if` / `else`.

**§ 30(2)** This is not a Sec 0.1 match form:

```sec
match ready {
    true => Start()
    false => Wait()
}
```

**§ 30(3)** Use:

```sec
if ready {
    Start()
} else {
    Wait()
}
```

**§ 30(4)** A union or enum variant may of course contain or expose a `bool` payload. The structural variant may be matched, after which the boolean may be inspected with a guard or ordinary `if`.

**§ 30(5)** `match` is justified by the structural or variant domain, not by the boolean value itself.

---

## § 31 Literal and range patterns are not part of Sec 0.1

**§ 31(1)** Ordinary literal value matching belongs to `switch` or `if`.

**§ 31(2)** Do not write:

```sec
match value {
    1 => One()
    2 => Two()
    _ => Other()
}
```

**§ 31(3)** Use `switch`.

**§ 31(4)** Range patterns are likewise not part of Sec 0.1:

```sec
match value {
    0..<10 => Small()
    _ => Other()
}
```

**§ 31(5)** Use the range facilities of `switch`.

**§ 31(6)** Enum member patterns are not classified as ordinary literal patterns merely because enum members have numeric representations.

---

## § 32 Unsupported pattern forms in Sec 0.1

**§ 32(1)** Sec 0.1 does not define:

- pattern alternatives such as `A | B`;
- general literal patterns;
- range patterns;
- direct `true` / `false` match patterns;
- runtime type patterns;
- ordinary struct-subject destructuring;
- nested recursive patterns, except the narrow `Err(ConcreteError.Variant)`
  Result/error refinement of § 28 and the open-error narrowing of § 29;
- arbitrary user-defined pattern protocols;
- regex patterns;
- guard syntax other than `where`.

**§ 32(2)** Write separate arms, nested matches, `switch`, or `if` as appropriate.

---

## § 33 No fallthrough

**§ 33(1)** Match arms never fall through.

**§ 33(2)** There is no `fallthrough` operation for `match`.

**§ 33(3)** Each selected arm either:

- completes its own body and reaches the match merge;
- produces the match expression value;
- returns;
- propagates;
- terminates through another defined control-flow operation.

---

## § 34 `break` and `continue`

**§ 34(1)** `match` is not a `break` or `continue` target.

**§ 34(2)** A `break` or `continue` written inside a match arm applies only when an enclosing loop or other construct legally provides that target.

**§ 34(3)** Before such control transfer, ordinary cleanup rules apply to scopes exited by the edge.

**§ 34(4)** The match construct itself does not create an implicit loop-like control target.

---

## § 35 Empty matches

**§ 35(1)** This is invalid in Sec 0.1:

```sec
match value {
}
```

**§ 35(2)** Even if a future type system introduces an uninhabited type, zero-arm match syntax is not part of Sec 0.1.

**§ 35(3)** The compiler should report one focused diagnostic.

---

## § 36 Compiler analysis requirements

**§ 36(1)** Before lowering, Sema and ownership/borrow analysis must resolve at least:

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

**§ 36(2)** No backend may infer these facts from emitted low-level code.

---

## § 37 LSP requirements

**§ 37(1)** The LSP is an interactive view of compiler-resolved match semantics.

**§ 37(2)** It must not implement a parallel match, ownership, borrow, or exhaustiveness analyzer.

**§ 37(3)** Compiler-produced facts must be available to the LSP early enough for normal interactive editing.

### § 37.1 Payload binding hover

**§ 37.1(1)** For a binding such as:

```sec
Some(value)
```

**§ 37.1(2)** hover must be able to expose at least:

```text
Type: Resource
Binding mode: move
Source: option.Some payload
Commit: when this arm is selected
```

**§ 37.1(3)** For a copyable payload:

```text
Binding mode: copy
```

**§ 37.1(4)** For borrowed payloads:

```text
Binding mode: shared borrow
```

**§ 37.1(5)** or:

```text
Binding mode: mutable borrow
```

### § 37.2 Guard-aware move display

**§ 37.2(1)** For a guarded move-only binding, tooling must distinguish a prospective arm move from an already committed move.

**§ 37.2(2)** Before a guarded arm is selected, the LSP must not display the subject as already moved merely because the pattern text contains a by-value move-only binding.

**§ 37.2(3)** It may display:

```text
Ownership effect: moves payload if this arm is selected
```

### § 37.3 Post-match availability

**§ 37.3(1)** When ownership merging produces a state such as `Moved`, `PartiallyAvailable`, or `ConditionallyAvailable`, hover and diagnostics must use the compiler's resolved state.

**§ 37.3(2)** A mandatory use-after-move or conditionally-unavailable error must be published by the LSP as the same language-safety diagnostic produced by the compiler.

**§ 37.3(3)** Tooling should identify:

- the arm that caused the move;
- the binding receiving ownership;
- the affected subject or payload Place;
- the merge that makes later use unavailable;
- valid borrow/reinitialization alternatives when provable.

**§ 37.3(4)** Configurable ownership inlay hints may show non-obvious `copy`, `move`, `ref`, and `ref mut` effects.

**§ 37.3(5)** The underlying ownership correctness is mandatory even when visual hints are disabled.

---

## § 38 Semantic IR requirements

**§ 38(1)** Resolved match semantics must be explicit before target lowering.

**§ 38(2)** Semantic IR must preserve or encode:

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

**§ 38(3)** Payload extraction or borrowing must occur only after the appropriate variant has been established.

**§ 38(4)** A move from a reusable subject through a guarded by-value binding must not be represented as committed on the guard-false edge.

**§ 38(5)** The backend must not perform match exhaustiveness analysis or reconstruct source ownership policy.

---

## § 39 Diagnostics

**§ 39(1)** Match diagnostics should be specific and source-related.

**§ 39(2)** Required categories include:

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

**§ 39(3)** Ownership diagnostics should include related locations for the source Place, move-producing arm, and later invalid use when available.

---

## § 40 Design summary

**§ 40(1)** Sec 0.1 `match` follows these rules:

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
