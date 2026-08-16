# While Statements

- **Status:** Normative
- **Created:** 2026-08-16
- **Last updated:** 2026-08-16
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/control-flow/flowcontrol_while.md`
- **Replaces:** `rules/control-flow/flowcontrol_while.txt`
- **Repository baseline reviewed:** `56be75d`

## 1. Purpose

`while` is used for condition-controlled repetition.

`for` is used for iteration over ranges, collections, maps, iterators, or for explicit unconditional repetition.

Sec intentionally does not use a condition-only `for` form.

## 2. Syntax

```sec
while condition {
    statements
}
```

Braces are mandatory.

Parentheses around the condition are permitted but normally unnecessary.

## 3. Condition type

The `while` condition must have type `bool`.

Sec has no implicit truthiness.

Valid:

```sec
while running {
    Poll()
}
```

```sec
while count < limit {
    count += 1
}
```

Invalid:

```sec
while 1 {
}
```

```sec
while items {
}
```

## 4. Condition evaluation

The condition is evaluated before every possible iteration.

The body executes only when the condition evaluates to `true`.

A normal `while` body may execute zero times.

The condition is evaluated exactly once for each attempted iteration and is not reevaluated during the same iteration.

## 5. Boolean operators and short-circuit evaluation

Normal boolean-expression rules apply.

`!`, `&&`, and `||` retain their ordinary Sec semantics.

`&&` and `||` use guaranteed left-to-right short-circuit evaluation.

```sec
while ready && CanContinue() {
    Work()
}
```

`CanContinue()` is evaluated only when `ready` is `true`.

Short-circuit behavior is language semantics and may be relied upon for safety and side-effect control.

## 6. Side effects in conditions

A condition may contain calls or other side effects when those operations are otherwise legal.

```sec
while ReadNext() {
    Process()
}
```

The operation is evaluated once before each possible iteration.

## 7. `try` in a condition

A `try` expression may occur in the condition when its successful value has type `bool`.

```sec
while try ReadStatus() {
    Poll()
}
```

This is valid when `ReadStatus()` uses ordinary Sec fallible semantics with a successful `bool` value.

A `Result` value itself is not implicitly boolean.

A `try` expression whose successful value is not `bool` is invalid as the condition.

## 8. `is` state tests

A `while` condition may use a non-binding `is` state test when another canonical rulebook defines that expression as producing `bool`.

The `while` statement does not introduce structural or payload-binding pattern syntax.

For example, a state test may be valid where its owning rulebook defines it:

```sec
while state is Active {
    Poll()
}
```

Pattern binding is invalid:

```sec
while Some(value) := current {
}
```

Use `match` or restructure the loop when payload destructuring is required.

## 9. No declarations in the header

Sec 0.1 does not permit declaration syntax inside a `while` header.

Invalid:

```sec
while let value := Read(); value > 0 {
}
```

Declarations must occur before the loop or inside the loop body.

## 10. Assignment is not an expression

Assignment cannot be used as a `while` condition.

Invalid:

```sec
while running = true {
}
```

Use ordinary assignment separately and then test a boolean expression.

## 11. Comparisons and membership

Normal comparison, equality, and membership rules apply.

Comparison chaining is not supported.

Invalid:

```sec
while 0 <= value < 100 {
}
```

Use:

```sec
while value >= 0 && value < 100 {
    ...
}
```

or preferably when appropriate:

```sec
while value in 0..<100 {
    ...
}
```

## 12. Loop progression

`while` does not provide automatic progression.

The body is responsible for changing any state needed for eventual termination.

```sec
let mut index: int := 0

while index < limit {
    Process(index)
    index += 1
}
```

The compiler may diagnose obviously non-progressing loops through analysis, but progression is not part of `while` syntax.

## 13. `break`

`break` exits the nearest enclosing loop.

```sec
while running {
    if ShouldStop() {
        break
    }

    Work()
}
```

`break` does not take a value in Sec 0.1.

## 14. `continue`

`continue` ends the current iteration and begins the next evaluation of the nearest enclosing loop's condition.

```sec
while running {
    if ShouldSkip() {
        continue
    }

    Work()
}
```

`continue` does not take a value in Sec 0.1.

## 15. Nested loops

Nested loops have independent control-flow targets.

A `break` or `continue` applies to the nearest enclosing loop.

```sec
while rowsRemain {
    while columnsRemain {
        if DoneWithRow() {
            break
        }

        ProcessCell()
    }
}
```

The inner `break` does not exit the outer loop.

## 16. No labeled loop control in Sec 0.1

Labeled `break` and `continue` are not part of Sec 0.1.

Invalid:

```sec
outer:
while running {
    while active {
        break outer
    }
}
```

Control flow that needs to exit multiple loop levels must be expressed explicitly through ordinary state, return/error propagation, or refactoring.

## 17. Interaction with `switch`

`switch` does not own loop `break` or `continue` targets.

When a `switch` is nested inside a `while`, an unlabelled `break` exits the surrounding loop.

```sec
while running {
    switch status {
    case Status.Stop:
        break
    }
}
```

An unlabelled `continue` begins the next evaluation of the surrounding `while` condition.

```sec
while running {
    switch status {
    case Status.Skip:
        continue

    default:
        Work()
    }
}
```

`switch` case termination does not require `break`.

## 18. Infinite loops

`while true` is legal.

```sec
while true {
    Poll()
}
```

The canonical explicit form for intentionally unconditional repetition is:

```sec
for {
    Poll()
}
```

Both forms are legal.

Use `for {}` when unconditional repetition is the direct programmer intent.

## 19. Constant conditions

Compile-time constant boolean conditions are legal.

```sec
while true {
    ...
}
```

```sec
while false {
}
```

A constant condition does not bypass the requirement that the condition type is `bool`.

Statements proven unreachable by a constant condition are diagnosed according to the canonical Sec unreachable-code policy.

Optimization may remove unreachable bodies or simplify constant loops after semantic analysis.

## 20. Non-continuing `while true`

A `while true` loop with no reachable exit path is non-continuing.

```sec
fn RunForever() int {
    while true {
        Poll()
    }
}
```

Control-flow analysis may therefore treat the end of the function as unreachable.

A reachable `break` introduces a possible continuing path after the loop.

```sec
fn MayStop(stop: bool) int {
    while true {
        if stop {
            break
        }
    }

    return 0
}
```

A `break` inside a nested loop does not make an outer `while true` loop continuing.

## 21. Return and propagation

`return` immediately exits the current function.

Ordinary error propagation and `try` semantics apply inside a `while` body.

```sec
while running {
    let item := try Read()
    Process(item)
}
```

If propagation exits the function, the loop does not continue.

## 22. Scope

The loop body creates a lexical block scope.

Declarations inside the body are not visible after the body.

Normal Sec rules for mutability, ownership, borrowing, no-shadowing, and destruction apply.

A new body scope is entered for every executed iteration.

## 23. Definite assignment

Assignments inside a normal `while` body do not by themselves make a variable definitely assigned after the loop because the loop may execute zero times.

```sec
let value: int

while condition {
    value = 10
    break
}

Use(value)
```

is invalid when the initial false path can reach `Use`.

General flow analysis may mark a value definitely assigned only when every reachable path that continues after the loop proves that assignment.

Non-continuing loop paths do not contribute to the post-loop merge.

## 24. No `while ... else`

Sec 0.1 does not support Python-style `while ... else`.

Invalid:

```sec
while condition {
    Work()
} else {
    Finished()
}
```

Use explicit state when behavior depends on why a loop ended.

## 25. No `do-while`

Sec 0.1 does not support `do-while` syntax.

When at-least-once execution is required, express the first iteration explicitly or restructure the loop.

## 26. No loop expression value

A `while` statement does not produce a value.

`break` and `continue` do not carry values.

Use an explicit variable or another control-flow construct when a computed value must survive the loop.

## 27. Parser requirements

The parser must:

- recognize `while condition { ... }`;
- require a condition before the body;
- require a block body;
- preserve source positions for the keyword, condition, and body;
- reject declaration syntax in the header;
- reject assignment where an expression is required;
- reject `while ... else`;
- reject `do-while` syntax;
- reject labeled loop control in Sec 0.1.

Expression-level `try`, `is`, logical, comparison, and membership semantics remain owned by their corresponding expression/type rules.

## 28. Sema and flow-analysis requirements

Sema/flow analysis must:

- resolve the condition expression;
- require the final condition type to be `bool`;
- reject implicit truthiness;
- preserve ordinary short-circuit semantics;
- permit `try` only when the successful condition value is `bool`;
- permit non-binding `is` tests when defined by another canonical rulebook;
- reject declaration and pattern-binding conditions;
- create the loop body scope;
- track the nearest active loop for `break` and `continue`;
- model `continue` as a transfer to the next condition evaluation;
- conservatively include the zero-iteration path for normal loops;
- exclude non-continuing paths from continuation merges;
- treat `while true` without a reachable exit as non-continuing;
- distinguish breaks in nested loops from breaks of the current loop;
- feed proven unreachable statements into the canonical unreachable-code diagnostic system.

## 29. CFG requirements

The semantic CFG for a normal `while` has conceptual regions equivalent to:

```text
entry
  -> condition

condition true
  -> body

body normal continuation
  -> condition

continue
  -> condition

condition false
  -> after-loop

break
  -> after-loop
```

Physical backend block structure may differ as long as these semantics are preserved.

## 30. Required diagnostics

Diagnostics should be specific.

Examples:

```text
while condition must have type bool, got int
```

```text
assignment is not allowed in while condition
```

```text
declaration is not allowed in while condition
```

```text
pattern binding is not allowed in while condition; use match
```

```text
try expression in while condition must produce bool, got Configuration
```

```text
break is only valid inside a loop
```

```text
continue is only valid inside a loop
```

```text
labeled break and continue are not supported in Sec 0.1
```

```text
comparison chaining is not supported; use && or a range
```

## 31. Best practice

Use `while` when repetition is controlled by a changing boolean condition.

Use `for` for iteration over ranges or collections.

Use `for {}` for intentionally unconditional repetition.

Prefer range membership when it expresses bounds more directly.

```sec
while value in 0..<100 {
    ...
}
```

Use short-circuit expressions when later evaluation depends on an earlier safety/state check.

## 32. Cross-rulebook ownership

This rulebook owns:

- `while` syntax;
- condition-controlled repetition;
- condition evaluation timing;
- `break`/`continue` behavior within `while`;
- `while` flow/definite-assignment behavior;
- the absence of `while ... else`, `do-while`, labeled loop control, and loop values in Sec 0.1.

Other rulebooks own:

- logical operators and precedence;
- comparison and membership;
- `is` state semantics;
- `try` and error propagation;
- `for` iteration and `for {}`;
- `switch` case semantics;
- ownership/borrowing/destruction;
- canonical unreachable-code diagnostics.
