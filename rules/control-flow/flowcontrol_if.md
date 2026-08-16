# If Statements

- **Status:** Normative
- **Created:** 2026-08-16
- **Last updated:** 2026-08-16
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/control-flow/flowcontrol_if.md`
- **Replaces:** `rules/control-flow/flowcontrol_if.txt`
- **Repository baseline reviewed:** `0f92cf4`

## 1. Purpose

`if` selects control flow using a boolean condition.

It does not introduce truthiness, declaration syntax, assignment expressions, or structural pattern binding.

## 2. Syntax

```sec
if condition {
    statements
}
```

```sec
if condition {
    statements
} else {
    statements
}
```

```sec
if condition {
    statements
} else if condition {
    statements
} else {
    statements
}
```

Braces are mandatory.

Parentheses around the condition are permitted but normally unnecessary.

The formatter removes redundant outer parentheses while preserving parentheses required for grouping.

`else if` is the canonical chained form.

Sec does not introduce `elseif` or `elif`.

## 3. Condition type

Every `if` condition must have type `bool`.

Sec has no implicit truthiness.

Values such as numeric values, strings, collections, enums, structs, `Option`, `Result`, references, and raw pointers are not implicitly converted to `bool`.

Valid:

```sec
if ready {
    Start()
}
```

```sec
if count > 0 {
    Process()
}
```

Invalid:

```sec
if count {
    Process()
}
```

Invalid:

```sec
if items {
    Process()
}
```

## 4. Boolean operators

The logical operators are:

```text
!
&&
||
```

`!` requires a `bool` operand.

Both operands of `&&` and `||` must have type `bool`.

Operator precedence relevant to ordinary conditions is:

```text
!
comparison / equality / membership
&&
||
```

Parentheses may make grouping explicit.

## 5. Short-circuit evaluation

`&&` and `||` use guaranteed left-to-right short-circuit evaluation.

For:

```sec
left && right
```

`right` is evaluated only when `left` is `true`.

For:

```sec
left || right
```

`right` is evaluated only when `left` is `false`.

This is language semantics and may be relied upon for safety and side-effect control.

Example:

```sec
if index < items.Len && items[index].Ready {
    Process(items[index])
}
```

The indexed access is not evaluated when `index >= items.Len`.

## 6. Condition evaluation order

An `if` condition is evaluated exactly once whenever execution reaches that statement.

Expression evaluation follows the normal Sec left-to-right evaluation rules.

For an `if` / `else if` chain:

- conditions are evaluated in source order;
- a later condition is evaluated only when all earlier conditions were false;
- the first matching branch executes;
- remaining conditions are skipped;
- `else` executes only when every preceding condition was false.

## 7. Comparisons and equality

Normal Sec comparison and equality rules apply inside conditions.

Examples:

```sec
if value == expected {
    ...
}
```

```sec
if temperature < maximum {
    ...
}
```

No implicit numeric, semantic-type, unit, or enum conversion is introduced merely because an expression appears in an `if`.

Comparison chaining is not supported.

Invalid:

```sec
if 0 <= value < 100 {
    ...
}
```

Use:

```sec
if value >= 0 && value < 100 {
    ...
}
```

or preferably when appropriate:

```sec
if value in 0..<100 {
    ...
}
```

## 8. Membership

A membership expression may be used when it produces `bool`.

Examples:

```sec
if value in minimum..maximum {
    ...
}
```

```sec
if key in values {
    ...
}
```

Range and collection membership follow their own canonical type and compatibility rules.

A type being iterable does not automatically make it a valid membership container.

## 9. Calls, fields, and properties

Any expression that produces `bool` may be used as a condition.

Examples:

```sec
if IsReady() {
    ...
}
```

```sec
if connection.Ready {
    ...
}
```

```sec
if !items.IsEmpty {
    ...
}
```

A non-boolean expression must participate in an operation that produces `bool`.

Example:

```sec
if items.Len > 0 {
    ...
}
```

## 10. `Option` and `Result`

`Option[T]` and `Result[T, E]` are not implicitly boolean.

Invalid:

```sec
if optionalValue {
    ...
}
```

Invalid:

```sec
if result {
    ...
}
```

Use the explicit operations, state tests, `match`, or error-handling mechanisms defined by their respective rulebooks.

`if` itself does not unwrap an `Option` or `Result`.

## 11. `try` in a condition

A `try` expression may occur inside an `if` condition when its successful expression type is `bool`.

Example:

```sec
if try HasPermission(user, resource) {
    Allow()
}
```

This is valid when the operation produces the ordinary Sec fallible form whose successful value is `bool`.

Normal `try` propagation rules apply.

Invalid when the successful value is not boolean:

```sec
if try LoadConfiguration() {
    ...
}
```

when the successful value is `Configuration`.

## 12. State tests with `is`

`if` may use an `is` expression when another canonical rulebook defines that expression as a boolean state test.

Examples include foreign/union state tests such as:

```sec
if value is Active {
    ...
}
```

```sec
if value is empty {
    ...
}
```

and, only in the unsafe foreign/raw-pointer context defined by FFI:

```sec
unsafe {
    if raw is null {
        ...
    }
}
```

The `if` statement does not invent the states or the semantics of `is`; the owning type/FFI rulebook does.

A state test does not introduce a payload binding.

## 13. No pattern binding in `if`

Structural or payload-binding patterns are not part of Sec 0.1 `if` syntax.

Invalid:

```sec
if Some(user) := optionalUser {
    ...
}
```

Invalid:

```sec
if result is Ok(value) {
    ...
}
```

Use `match` when a variant payload or structural pattern must be destructured.

This rule does not prohibit non-binding boolean `is` state tests defined by another rulebook.

## 14. No declaration syntax in the header

Sec 0.1 does not introduce declarations or initializer clauses inside an `if` header.

Invalid:

```sec
if let user := FindUser(id); user.Active {
    ...
}
```

Invalid:

```sec
if user := FindUser(id) {
    ...
}
```

Declare values before the `if`.

```sec
let user := FindUser(id)

if user.Active {
    ...
}
```

## 15. Assignment is not an expression

Assignment is not an expression and therefore cannot be an `if` condition.

Invalid:

```sec
if ready = true {
    ...
}
```

Use:

```sec
ready = true

if ready {
    ...
}
```

Equality remains legal:

```sec
if ready == true {
    ...
}
```

but is normally redundant.

## 16. No implicit narrowing or destructuring

An `if` comparison or state test does not automatically create a new binding or unwrap a value.

For example, testing an optional value for absence/presence does not itself create the contained `T` binding.

Use the canonical `Option`, union, or `match` operation when payload access requires destructuring.

Any type refinement explicitly defined by the owning type rulebook remains valid; `if` does not create additional narrowing rules.

## 17. Branch scopes

Every branch body creates its own lexical block scope.

A declaration in one branch is not visible in another branch or after the statement.

Outer bindings remain visible according to normal scope rules.

Mutation inside a branch follows ordinary mutability, borrowing, ownership, and contract rules.

## 18. Definite assignment

Flow analysis merges definite-assignment state from all continuing branches.

A binding is definitely assigned after an `if` only when every continuing path reaching the continuation has assigned it.

Example:

```sec
let result: int

if condition {
    result = 1
} else {
    result = 2
}

Use(result)
```

Without an `else`, the false path remains possible unless flow analysis proves otherwise.

Branches that return, propagate, terminate, or otherwise do not continue are excluded from the continuation merge.

## 19. Terminating branches

Branches participate in ordinary control-flow analysis.

A branch that always:

- returns;
- propagates an error;
- terminates;
- or otherwise cannot continue

does not contribute a continuing path after the `if`.

## 20. Constant conditions and unreachable code

Literal and compile-time constant boolean conditions are syntactically and semantically valid conditions.

Examples:

```sec
if true {
    ...
}
```

```sec
if false {
}
```

Compile-time evaluation does not change the requirement that the condition has type `bool`.

When a compile-time condition proves statements unreachable, the general Sec unreachable-code policy applies.

A proven unreachable statement is diagnosed according to that canonical policy.

An empty unreachable branch contains no unreachable statement merely by existing.

Optimization may remove unreachable branches after semantic analysis.

## 21. Empty branches

Empty branches are valid.

```sec
if ready {
}
```

```sec
if ready {
} else {
}
```

Style/lint analysis may report an empty branch when configured, but emptiness is not itself a semantic error.

## 22. Side effects

A condition may contain side effects when those operations are otherwise legal.

Short-circuit and evaluation-order guarantees still apply.

Example:

```sec
if cached || LoadCache() {
    ...
}
```

Whether a side effect is good style is an analysis/style question, not an `if` semantic restriction.

## 23. Raw pointers and foreign null

Raw pointers are not implicitly boolean.

Invalid:

```sec
if raw {
    ...
}
```

Pointer validity and nullability use the raw-pointer/FFI rules.

In particular, `null` is not a normal value and may be tested only in the unsafe context required by FFI:

```sec
unsafe {
    if raw is null {
        ...
    }
}
```

The `if` statement itself does not create pointer safety or an unsafe context.

## 24. Match expressions

A `match` expression may be used as an `if` condition when the complete expression has type `bool`.

```sec
if match state {
    State.Ready => true
    State.Waiting => false
    State.Failed => false
} {
    Start()
}
```

Normal `match` exhaustiveness and expression-typing rules apply.

A non-value-producing `match` statement is not an `if` condition.

## 25. Formatter requirements

Canonical formatting is:

```sec
if condition {
    ...
} else if otherCondition {
    ...
} else {
    ...
}
```

The formatter:

- keeps braces;
- places `else` on the same line as the preceding closing brace;
- removes redundant outer condition parentheses;
- preserves parentheses needed for grouping.

## 26. Parser requirements

The parser must:

- parse `if`, `else if`, and `else` chains;
- require a block for every branch;
- permit ordinary expression grammar as the condition;
- preserve source positions for each keyword, condition, and block;
- reject declaration syntax in the header;
- reject assignment where an expression is required;
- leave expression-level `is`, `try`, comparison, membership, and logical semantics to their owning grammar/sema rules.

No separate `elseif` or `elif` keyword exists.

## 27. Sema and flow-analysis requirements

Sema/flow analysis must:

- require the condition result type to be `bool`;
- reject implicit truthiness;
- validate `!`, `&&`, and `||` operands as boolean;
- preserve left-to-right short-circuit semantics;
- apply ordinary equality/comparison/membership rules;
- apply ordinary call/property/indexing rules;
- permit `try` only when the resulting successful value is `bool`;
- permit non-binding `is` tests when defined by the owning rulebook;
- reject pattern-binding conditions;
- create independent branch scopes;
- analyze branch continuation/termination;
- merge definite-assignment state over continuing paths;
- preserve a false/no-branch path when no `else` exists;
- feed proven unreachable statements into the canonical unreachable-code diagnostic system.

## 28. Required diagnostics

Diagnostics should be specific.

Examples:

```text
if condition must have type bool, got int
```

```text
left operand of && must have type bool, got int
```

```text
right operand of || must have type bool, got string
```

```text
operator ! requires bool operand, got int
```

```text
assignment is not allowed in if condition
```

```text
declaration is not allowed in if condition
```

```text
pattern binding is not allowed in if condition; use match
```

```text
try expression in if condition must produce bool, got Configuration
```

```text
comparison chaining is not supported; use && or a range
```

```text
variable result may be unassigned after if statement
```

## 29. Best practice

Prefer direct boolean expressions.

```sec
if ready {
    ...
}
```

rather than:

```sec
if ready == true {
    ...
}
```

Prefer range membership when it expresses the condition more directly.

```sec
if value in 0..<100 {
    ...
}
```

rather than:

```sec
if value >= 0 && value < 100 {
    ...
}
```

Use `match` for payload destructuring and structural pattern matching.

Use short-circuit operators when later evaluation depends on an earlier safety or state check.

## 30. Cross-rulebook ownership

This rulebook owns:

- `if` / `else if` / `else` syntax;
- boolean condition requirement;
- branch evaluation order;
- `if`-chain control flow;
- branch scope and flow merging;
- interaction of `if` with already-defined boolean expressions.

Other rulebooks own:

- logical operator definitions and precedence;
- equality and ordered comparison;
- range and collection membership;
- `is` state semantics;
- `Option` and `Result` operations;
- `try`;
- `match` patterns and destructuring;
- raw-pointer and `null` semantics;
- ownership, borrowing, and mutation;
- canonical unreachable-code diagnostics.
