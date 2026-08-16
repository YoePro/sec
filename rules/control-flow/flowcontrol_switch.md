# Switch Statements

- **Status:** Normative
- **Created:** 2026-08-16
- **Last updated:** 2026-08-16
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/control-flow/flowcontrol_switch.md`
- **Replaces:** `rules/control-flow/flowcontrol_switch.txt`
- **Repository baseline reviewed:** `56be75d`

## 1. Purpose

`switch` provides ordered value-based or condition-based branching.

Sec supports:

- switch with a subject expression;
- subjectless switch.

`switch` is not structural pattern matching.

Use `match` for:

- enum or union exhaustiveness;
- variant payload binding;
- `Result` and `Option` destructuring;
- structural patterns;
- guarded patterns.

`switch` is a statement in Sec 0.1.

It does not produce a value.

## 2. Subject switch syntax

```sec
switch expression {
case value:
    statements

case value1, value2, value3:
    statements

case range:
    statements

case relationalCase:
    statements

default:
    statements
}
```

A switch body may contain zero or more `case` clauses followed by an optional `default`.

`default`, when present, must be unique and final.

## 3. Subjectless switch syntax

```sec
switch {
case condition:
    statements

case otherCondition:
    statements

default:
    statements
}
```

Every case item in a subjectless switch must have type `bool`.

A subjectless switch is an ordered condition chain similar to `if` / `else if`.

## 4. Subject evaluation

The subject expression is evaluated exactly once.

```sec
switch ReadStatus() {
case Status.Ready:
    Start()

default:
    Stop()
}
```

`ReadStatus()` is called exactly once.

The resulting value is used for all case tests.

The compiler may keep it in an SSA value, register, or temporary storage according to lowering needs.

## 5. Case evaluation order

Cases are considered from top to bottom in source order.

Within a case, comma-separated case items are considered from left to right.

The first matching case body executes.

After a case matches:

- later case tests are not evaluated;
- the case body executes once;
- normal completion continues after the switch;
- explicit `fallthrough` may instead enter the next case body.

Dynamic case expressions are evaluated only when execution reaches them.

Their side effects therefore follow source order.

## 6. Value cases

A value case compares the subject with one or more equality-compatible expressions.

```sec
switch value {
case 1:
    One()

case 2, 3, 5, 7:
    Prime()

default:
    Other()
}
```

Comma-separated items are alternatives.

Conceptually:

```sec
case 2, 3, 5, 7:
```

matches when the subject equals any listed value.

Each item must be equality-compatible with the subject according to the ordinary Sec type rules.

No implicit truthiness or unrelated type conversion is introduced.

## 7. Case expressions

A value case expression may be any expression that is valid in the case position and equality-compatible with the subject.

Examples include:

- literals;
- constants;
- variables;
- fields;
- properties;
- function calls;
- method calls;
- other compatible expressions.

Example:

```sec
switch value {
case minimum:
    AtMinimum()

case CalculateTarget():
    AtTarget()
}
```

Case expressions are evaluated lazily in source order.

## 8. Range cases

A subject switch may contain range items.

```sec
switch score {
case 0..<50:
    Failed()

case 50..<80:
    Passed()

case 80..100:
    Excellent()
}
```

Supported range forms follow the canonical Sec range rules.

Examples:

```sec
start..end
start..<end
start..
..end
..<end
```

The subject and range bounds must have compatible ordered types.

A range case is a membership test, not iteration.

No sequence is constructed and no repeated subject evaluation occurs.

Normalized-range semantics follow the canonical range rulebook.

## 9. Mixed values and ranges

One case may contain several compatible value and range alternatives.

```sec
switch value {
case 1, 3, 5, 10..<20:
    Selected()

default:
    Other()
}
```

The alternatives are tested left to right.

The body executes once when any item matches.

## 10. Relational cases

A subject switch may use a relational operator without repeating the subject.

Supported forms are:

```sec
case < expression:
case <= expression:
case > expression:
case >= expression:
```

Example:

```sec
switch value {
case < 0:
    Negative()

case 0:
    Zero()

case > 0:
    Positive()
}
```

This:

```sec
switch value {
case < limit:
    Below()
}
```

uses the same ordered-comparison semantics as:

```sec
if value < limit {
    Below()
}
```

The subject and relational operand must support the requested ordered comparison.

## 11. Multiple relational alternatives

Relational alternatives may be combined in one case.

```sec
switch value {
case < minimum, > maximum:
    Outside()

default:
    Inside()
}
```

The alternatives are evaluated left to right.

They are alternatives, not a conjunction.

## 12. Subjectless switch

A subjectless switch evaluates boolean case expressions.

```sec
switch {
case score < 0:
    Invalid()

case score < 50:
    Failed()

case score < 80:
    Passed()

default:
    Excellent()
}
```

Every case item must have type `bool`.

There is no implicit truthiness.

Invalid:

```sec
switch {
case 10:
    Invalid()
}
```

when `10` has an integer type.

## 13. Subjectless comma alternatives

Comma-separated conditions in a subjectless case mean boolean OR with left-to-right short-circuit behavior.

```sec
switch {
case user.IsAdmin(), user.IsOwner():
    Allow()

case user.IsBlocked():
    Deny()

default:
    Review()
}
```

Once one condition in the case evaluates to `true`, later conditions in that case are not evaluated.

Once a case matches, later cases are not evaluated.

## 14. `default`

A switch may contain at most one `default`.

`default` executes only when no preceding case matches.

```sec
switch value {
case 1:
    One()

case 2:
    Two()

default:
    Other()
}
```

`default` is optional.

If no case matches and no `default` exists, execution continues after the switch.

`default` must be the final switch clause.

This keeps ordered case evaluation and explicit fallthrough behavior unambiguous.

## 15. No implicit fallthrough

A normal case body does not continue into the next case body.

```sec
switch value {
case 1:
    One()

case 2:
    Two()
}
```

When `value` is `1`, only `One()` executes.

Normal completion then continues after the switch.

No terminator is required at the end of an ordinary case.

## 16. Explicit `fallthrough`

`fallthrough` explicitly transfers control from the current case body to the body of the immediately following case.

```sec
switch value {
case 1:
    One()
    fallthrough

case 2:
    OneOrTwo()

default:
    Other()
}
```

When `value` is `1`:

1. `One()` executes;
2. `fallthrough` transfers directly to the next case body;
3. the next case's tests are not evaluated;
4. `OneOrTwo()` executes.

`fallthrough` must be the final non-empty statement in the current case.

It is invalid in the final case.

It is invalid in `default`, because `default` is final.

It is valid only directly inside the switch case body whose successor it targets.

A nested `if`, loop, function, lambda, or other nested block cannot use `fallthrough` to target an enclosing switch case.

## 17. Empty cases

An empty case is legal.

```sec
switch value {
case 1:

case 2:
    Two()
}
```

An empty case does not implicitly fall through.

When `value` is `1`, no statements execute and control continues after the switch.

Use explicit `fallthrough` when entering the next case body is intended.

## 18. Case scopes

Every case body has its own lexical scope.

```sec
switch value {
case 1:
    let message: string := "one"
    Print(message)

case 2:
    let message: string := "two"
    Print(message)
}
```

The two `message` bindings belong to independent sibling scopes.

A binding declared in one case is not visible in another case or after the switch.

Outer bindings remain visible according to the normal scope rules.

Normal no-shadowing, ownership, borrowing, mutability, and contract rules apply.

## 19. Fallthrough and scope

`fallthrough` transfers control into the next case body, but it does not merge the lexical scopes of the two cases.

Bindings declared in the source case are not visible by name in the destination case merely because control arrived through `fallthrough`.

The destination case executes under its own lexical scope.

Ownership and destruction analysis must account for control leaving the source case and entering the destination case.

## 20. Duplicate compile-time values

Two compile-time case values that are equal under the subject's equality semantics are invalid.

```sec
switch value {
case 1:
    First()

case 1:
    Second()
}
```

The compiler must diagnose the duplicate rather than relying on source-order selection.

The same principle applies to duplicate alternatives inside a single case.

## 21. Overlapping compile-time ranges

Compile-time range cases must not overlap when the compiler can prove the overlap.

Invalid:

```sec
switch score {
case 0..50:
    First()

case 40..100:
    Second()
}
```

A compile-time value may not be covered by an earlier compile-time range.

Invalid:

```sec
switch score {
case 0..100:
    InRange()

case 50:
    Exact()
}
```

The compiler must diagnose the unreachable later item.

## 22. Relational reachability

Relational cases may overlap partially when the later case still has values that can reach it.

Valid:

```sec
switch value {
case <= -10:
    VeryLow()

case < 0:
    Low()
}
```

The second case remains reachable for values from `-9` through `-1`.

A later relational case that is fully covered by earlier cases is unreachable and must be diagnosed.

Invalid:

```sec
switch value {
case >= 0:
    NonNegative()

case > 10:
    Large()
}
```

## 23. Dynamic overlap

When values, range bounds, or relational operands are dynamic, complete overlap may not be statically provable.

Such cases remain valid unless the compiler can prove that a later case is unreachable.

Runtime source order determines which matching case wins.

## 24. Enum subjects

An enum value may be used as a switch subject when ordinary equality semantics permit it.

```sec
switch direction {
case Direction.North:
    North()

case Direction.South:
    South()

default:
    Other()
}
```

A switch over an enum is not required to be exhaustive.

Use `match` when exhaustive handling of the enum domain is required.

Analysis may report omitted known enum values when no `default` exists, but non-exhaustiveness is not by itself a switch semantic error.

For open foreign or hardware enum domains, a `default` is generally required by the owning enum/FFI semantics when unknown values must be handled.

## 25. String subjects

Strings may be switch subjects when ordinary string equality is valid.

```sec
switch command {
case "start":
    Start()

case "stop":
    Stop()

default:
    Unknown()
}
```

The switch statement does not introduce special string comparison semantics.

## 26. Named semantic types and units

Named semantic types retain their ordinary compatibility rules.

```sec
switch speed {
case minimumSpeed..maximumSpeed:
    Normal()
}
```

Plain unrelated primitive values do not become compatible merely because they appear in a switch.

Unit-bearing values retain the normal unit compatibility rules.

```sec
switch distance {
case 0<m>..<100<m>:
    Near()

case 100<m>..:
    Far()
}
```

Incompatible units cannot participate in the same comparison or range test.

## 27. Boolean subjects

A `bool` value may be used as an explicit switch subject.

```sec
switch ready {
case true:
    Start()

case false:
    Wait()
}
```

This is valid.

An `if` / `else` is often simpler when only one boolean distinction is required, but the language does not prohibit boolean subjects.

## 28. No pattern matching

`switch` does not support structural or variant payload patterns.

Invalid forms include:

```sec
switch result {
case Ok(value):
    Use(value)
}
```

Use `match` when binding or destructuring a variant payload.

`switch` also does not introduce:

- wildcard patterns;
- property patterns;
- list patterns;
- type-binding patterns;
- structural destructuring.

## 29. No switch case guards

Sec 0.1 does not add a separate `where` guard syntax to switch cases.

Invalid:

```sec
switch value {
case 10..20 where user.Active:
    Handle()
}
```

Use a subjectless switch when the condition itself should determine the branch:

```sec
switch {
case value in 10..20 && user.Active:
    Handle()
}
```

or use `if` inside the selected case body.

`where` remains part of the canonical `match` guard syntax, not switch syntax.

## 30. Switch is not an expression

`switch` is a statement in Sec 0.1.

Invalid:

```sec
let result := switch value {
case 1:
    10

default:
    20
}
```

Use `match` for value-producing branching, or assign to an outer binding from the switch cases.

```sec
let mut result: int

switch value {
case 1:
    result = 10

default:
    result = 20
}
```

## 31. Loop-control statements

A switch does not create a `break` or `continue` target.

`break` and `continue` retain the loop-only semantics defined by the loop rulebooks and the canonical grammar.

They are not used to terminate ordinary switch cases or to enter the next case.

`fallthrough` is the only switch-specific case-to-case control-transfer statement.

## 32. Switch termination

A switch is terminating when every reachable execution path through it terminates rather than reaching the continuation after the switch.

This normally requires:

- complete branch coverage, typically through `default` or another statically proven complete subject domain;
- every reachable case body to terminate;
- every `fallthrough` chain to reach a terminating destination;
- no reachable unmatched continuation path.

Example:

```sec
fn Classify(value: int) int {
    switch value {
    case < 0:
        return -1

    case 0:
        return 0

    default:
        return 1
    }
}
```

This satisfies the function return requirement.

A switch without complete coverage normally leaves an unmatched continuation path.

## 33. Definite assignment

Definite-assignment analysis merges only paths that continue after the switch.

```sec
fn Select(value: int) int {
    let mut result: int

    switch value {
    case 1:
        result = 10

    case 2:
        result = 20

    default:
        result = 30
    }

    return result
}
```

This is valid because every possible continuing path assigns `result`.

Without complete coverage, an unmatched path must also be considered.

Fallthrough chains participate in the same control-flow merge.

## 34. Compile-time unreachable cases

When the compiler proves that a later case or case item can never be selected because previous cases completely cover it, the canonical unreachable-code policy applies.

The diagnostic should identify the earlier case or range that establishes the coverage when practical.

Optimization may simplify the final control-flow graph only after semantic reachability has been determined.

## 35. Parser requirements

The parser must support:

```text
switch [Expression] {
    case SwitchCaseItem [, SwitchCaseItem ...]:
        Statement*
    ...
    default:
        Statement*
}
```

Supported case-item syntax includes:

- ordinary expressions;
- range expressions;
- relational forms using `<`, `<=`, `>`, `>=`.

The parser must:

- permit subject and subjectless forms;
- permit comma-separated alternatives;
- permit at most one syntactic `default` candidate for later semantic validation;
- preserve source order;
- preserve source locations for case items and bodies;
- represent each case body as an independent lexical region;
- parse `fallthrough` as a statement for later context validation.

The parser does not perform subject/case type compatibility checks.

## 36. Sema requirements for subject switch

Sema must:

- resolve/evaluate the subject once in the semantic model;
- reject a subject that does not produce a usable value;
- validate value cases using ordinary equality compatibility;
- validate ranges using ordered compatibility and canonical range rules;
- validate relational cases using ordered comparison rules;
- validate every comma-separated alternative independently;
- detect duplicate compile-time values;
- detect statically provable overlapping ranges;
- detect values already covered by previous ranges;
- detect fully covered/unreachable relational cases;
- permit partially overlapping relational cases when reachable values remain;
- preserve source-order semantics for dynamic cases;
- create an independent scope for every case;
- validate `default`;
- validate `fallthrough`;
- analyze termination;
- analyze definite assignment;
- feed proven unreachable cases/statements into the canonical diagnostics system.

## 37. Sema requirements for subjectless switch

Sema must:

- require every case item to have type `bool`;
- use left-to-right short-circuit evaluation for comma-separated alternatives;
- evaluate cases in source order;
- create an independent scope for every case;
- validate `default`;
- validate `fallthrough`;
- analyze termination;
- analyze definite assignment;
- preserve ordinary ownership, borrowing, and effect semantics for evaluated conditions.

## 38. `fallthrough` validation

Sema must reject `fallthrough` when:

- it is not directly inside a switch case body;
- it is inside a nested block and attempts to target an enclosing case;
- it is not the final non-empty statement in the case;
- there is no following case;
- it appears in `default`.

A valid `fallthrough` transfers directly to the next case body without evaluating that case's items.

## 39. Lowering requirements

Lowering must preserve these source semantics:

- subject evaluated exactly once;
- case tests evaluated in source order;
- comma alternatives evaluated left to right;
- subjectless comma alternatives short-circuit;
- first matching case wins;
- normal case completion branches to the switch continuation;
- `fallthrough` branches directly to the next case body without evaluating its tests;
- `default` executes only after all previous tests fail;
- case lexical scopes and cleanup edges are preserved.

The backend may optimize constant switches into jump tables, decision trees, or other equivalent forms.

Such optimization must not change observable evaluation order where dynamic expressions or side effects exist.

## 40. Required diagnostics

Diagnostics should be specific.

Examples:

```text
switch case must be compatible with subject type int, got string
```

```text
switch range must be compatible with subject type Speed, got int
```

```text
relational switch case requires an ordered subject type
```

```text
subjectless switch case must have type bool, got int
```

```text
duplicate switch case value 10
```

```text
switch case range overlaps a previous case
```

```text
switch case value 50 is already covered by a previous case
```

```text
unreachable switch case; previous cases already cover this condition
```

```text
switch may contain only one default clause
```

```text
default must be the final switch clause
```

```text
fallthrough must be the final statement in a switch case
```

```text
fallthrough is not allowed in the final switch case
```

```text
fallthrough is valid only directly inside a switch case body
```

```text
switch does not support pattern binding; use match
```

```text
switch case guards are not part of Sec 0.1; use a subjectless switch, if, or match
```

```text
switch is a statement and does not produce a value
```

## 41. Best practice

Use `switch` when several ordered comparisons against one subject or several peer boolean conditions make the branch structure clearer than a long `if` chain.

Use `match` when:

- exhaustiveness is semantically important;
- variants or payloads must be bound;
- structural patterns are required;
- a value-producing branch expression is desired.

Prefer ranges when they state the intended interval directly.

Prefer relational cases when they avoid repeating the switch subject.

Use `fallthrough` only when the next case body must execute without re-testing that case.

Do not simulate C-style switch control flow.

## 42. Cross-rulebook ownership

This rulebook owns:

- subject and subjectless `switch`;
- switch case ordering;
- value/range/relational case selection;
- comma-separated case alternatives;
- `default` placement and behavior;
- no-implicit-fallthrough semantics;
- explicit `fallthrough`;
- switch case scopes;
- switch termination and definite-assignment merging.

Other rulebooks own:

- equality and ordered-comparison semantics;
- ranges;
- units and semantic-type compatibility;
- `if`;
- `match` patterns, guards, exhaustiveness, and value production;
- loop-only `break` and `continue`;
- ownership, borrowing, destruction, and cleanup;
- general unreachable-code policy;
- target/backend optimization strategy.
