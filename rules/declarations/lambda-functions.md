# Sec Lambda Functions and Closures

- **Status:** Normative
- **Created:** 2026-08-14
- **Last updated:** 2026-08-26
- **Document revision:** 2.0
- **Language version:** Sec 0.1
- **Replaces:** `rules/declarations/functions_lambda.txt`
- **Canonical path:** `rules/declarations/lambda-functions.md`

## 1. Purpose

A lambda is an anonymous function value.

A lambda uses the same parameter, return, ownership, call, variadic, and error-handling rules as an ordinary named function unless this rulebook explicitly defines closure-specific behavior.

A lambda may:

- be assigned to a variable;
- be passed as a function argument;
- be returned from a function;
- be stored in a field or collection when its callable type is permitted there;
- be called through a function value;
- explicitly capture values or borrows from an enclosing local scope.

A non-capturing lambda is an anonymous function value.

A capturing lambda is a closure.

Sec does not introduce a separate arrow-only lambda language.

## 2. Basic lambda syntax

A lambda expression uses `fn` without a function name.

```sec
let double := fn(value: int) int {
    return value * 2
}
```

Named function:

```sec
fn Add(left: int, right: int) int {
    return left + right
}
```

Lambda expression:

```sec
fn(left: int, right: int) int {
    return left + right
}
```

The lambda body is a normal function body.

A lambda expression may appear anywhere an expression is valid and the surrounding expression rules permit it.

## 3. Explicit parameter and return types

Lambda parameter types are explicit.

```sec
let positive := fn(value: int) bool {
    return value > 0
}
```

Contextual omission of the parameter type is not part of Sec 0.1.

Invalid:

```sec
let positive: fn(int) bool := fn(value) bool {
    return value > 0
}
```

Lambda return types are also explicit.

Invalid:

```sec
let identity := fn(value: int) {
    return value
}
```

Expected diagnostic:

```text
lambda return type is required
```

A lambda with no result uses `void`.

```sec
let notify := fn(message: string) void {
    Print(message)
}
```

## 4. Lambda parameters follow ordinary function rules

Lambda parameters use the same parameter grammar and semantics as named functions.

This includes:

```sec
fn(value: T) U {
    ...
}
```

```sec
fn(value: ref T) U {
    ...
}
```

```sec
fn(value: ref mut T) U {
    ...
}
```

```sec
fn(-> value: T) U {
    ...
}
```

and native typed variadics:

```sec
fn(values: ...T) U {
    ...
}
```

Owned by-value lambda parameters are mutable local bindings by default, exactly like owned parameters of named functions.

Borrowed parameters remain governed by `ref` and `ref mut`.

`->` parameter semantics remain forced-consuming by-value semantics.

`->` may not combine with `ref`, `ref mut`, or `...`.

## 5. Return semantics

Lambda return checking is identical to ordinary function return checking.

A non-`void` lambda returns exactly one value.

```sec
let operation := fn(value: int) int {
    return value * 2
}
```

A `void` lambda may use:

```sec
return
```

and may reach the end of its body when ordinary `void` function rules permit it.

Sec 0.1 does not support multiple lambda return values.

Related multiple values must be returned through one explicit type.

## 6. No expression-body shorthand

Lambda expressions always use the normal block body.

Invalid:

```sec
fn(value: int) int => value * 2
```

Invalid:

```sec
value => value * 2
```

Canonical:

```sec
fn(value: int) int {
    return value * 2
}
```

This keeps named and anonymous functions structurally aligned.

## 7. Function value types

A callable value has a statically known callable type.

Basic form:

```sec
fn(int) int
```

Function type parameter names are omitted.

Examples:

```sec
fn(int) int
fn(int, int) bool
fn(string) void
fn() bool
```

Callable parameter contracts are preserved in the type.

Examples:

```sec
fn(ref Buffer) void
fn(ref mut Buffer) void
fn(-> Buffer) void
fn(...int) int
```

The callable receiver/environment capability may prefix the function type.

```sec
fn(int) int
mut fn() int
-> fn() Resource
```

The two uses of `->` are distinct by position.

```sec
-> fn(-> Resource) Handle
```

means:

- the callable value itself is consuming;
- its parameter is also consuming.

## 8. Callable capability

Callable values have one of three environment/receiver capabilities:

```text
fn       shared/reusable callable
mut fn   mutable/exclusive callable
-> fn    consuming callable
```

A plain `fn` call does not require mutation or consumption of the callable environment.

A `mut fn` call requires mutable/exclusive access to the callable value because the invocation may mutate its environment.

A `-> fn` call consumes the callable value. The callable cannot be invoked again after a successful consuming call.

This capability is separate from parameter ownership modes.

## 9. Lambda callable capability is inferred

A lambda expression is always written with ordinary `fn`.

The programmer does not write:

```sec
mut fn(...) ...
```

or:

```sec
-> fn(...) ...
```

as the lambda expression introducer.

Sema infers callable capability from the lambda body and captures.

Example:

```sec
let factor := 2

let multiply := capture(factor) fn(value: int) int {
    return value * factor
}
```

The callable type is:

```sec
fn(int) int
```

Example:

```sec
let count := 0

let next := capture(count) fn() int {
    count += 1
    return count
}
```

The callable type is:

```sec
mut fn() int
```

Example:

```sec
let resource := OpenResource()

let take := capture(resource) fn() Resource {
    return resource
}
```

If returning `resource` moves it out of the closure environment, the callable type is:

```sec
-> fn() Resource
```

## 10. Callable capability compatibility

Callable capability is ordered by required authority.

A callable requiring less authority may be used where the target permits more authority.

Therefore:

```text
fn      -> may satisfy fn, mut fn, or -> fn target authority
mut fn  -> may satisfy mut fn or -> fn target authority
-> fn   -> may satisfy only -> fn target authority
```

The reverse conversions are invalid.

A `mut fn` cannot be treated as plain `fn`, because a plain callable may be invoked through shared access.

A `-> fn` cannot be treated as reusable `fn` or `mut fn`, because the invocation consumes the callable.

When a less-demanding callable is converted to a more-demanding callable type, the target contract may intentionally impose stricter use. For example, converting a reusable `fn` value to `-> fn` means the resulting callable value is consumed when called through that target.

No callable conversion may make an operation require less authority than the source callable actually needs.

## 11. Function type identity and compatibility

Callable type compatibility includes at least:

- callable capability;
- parameter count;
- ordered parameter types;
- parameter ownership/borrow modes;
- variadic shape and element type;
- return type.

Parameter names are not part of the callable type.

Return type is part of the callable type.

A callable assignment must preserve or safely strengthen callable authority according to the capability compatibility rule.

## 12. Named functions as callable values

A named function may be used as a callable value when one concrete overload and generic specialization can be resolved.

```sec
fn IsPositive(value: int) bool {
    return value > 0
}

let predicate: fn(int) bool := IsPositive
```

An overloaded function name requires enough target context to select one overload.

An unresolved overload set is not one runtime callable value.

An unresolved generic function template is not one runtime callable value.

## 13. Explicit capture syntax

Enclosing local bindings are never captured implicitly.

A capture clause appears immediately before the lambda's `fn`.

```sec
capture(factor) fn(value: int) int {
    return value * factor
}
```

A capture list is comma-separated and may have a trailing comma.

```sec
capture(
    minimum,
    maximum,
) fn(value: int) bool {
    return value in minimum..maximum
}
```

An empty capture clause is legal but redundant.

The formatter may remove:

```sec
capture() fn() void {
}
```

## 14. Capture names only

Capture entries identify visible local bindings.

Arbitrary capture expressions are not part of Sec 0.1.

Invalid:

```sec
capture(value + 1) fn() int {
    ...
}
```

Use a named local:

```sec
let offset := value + 1

let operation := capture(offset) fn() int {
    return offset
}
```

This keeps capture ownership and diagnostics tied to explicit bindings.

## 15. Capture forms

Sec 0.1 supports four capture forms:

```sec
capture(value)
capture(<-value)
capture(ref value)
capture(ref mut value)
```

They mean:

```text
value          owned copy capture
<-value        consuming owned capture
ref value      shared borrowed capture
ref mut value  exclusive mutable borrowed capture
```

Capture modes are explicit.

## 16. Ordinary owned capture

A plain capture requests an owned copy.

```sec
capture(value) fn() T {
    ...
}
```

For a copyable captured type, the closure receives an owned copy.

The outer binding remains valid.

For a non-copyable reusable source, plain capture is invalid; it never silently
changes into a destructive move.

The compiler must never silently clone a move-only value.

## 17. Consuming capture

A capture may explicitly transfer ownership with `<-`.

```sec
capture(<-value) fn() int {
    ...
}
```

This consumes the outer binding into the closure environment even if the value would otherwise be implicitly copyable.

`<-` on a capture affects closure construction and may intentionally consume a
copyable value. The old `capture(-> value)` spelling is invalid.

It does not automatically make the resulting callable a `-> fn`.

The callable becomes consuming only if invoking the lambda consumes closure environment state.

## 18. Owned captured bindings are mutable inside the closure

An owned capture is an owned field of the closure environment.

The captured local binding inside the lambda is mutable by default.

```sec
let count := 0

let next := capture(count) fn() int {
    count += 1
    return count
}
```

The outer `count` is unaffected when it was copied into the environment.

Mutating an owned captured binding normally requires `mut fn` callable capability because repeated calls mutate closure state.

No `capture(mut value)` syntax exists.

The compiler infers mutation from the body.

## 19. Shared reference capture

A shared borrowed capture uses:

```sec
capture(ref value) fn() int {
    return value
}
```

The closure stores shared borrowed authority according to the normal borrowing rules.

The closure must not outlive the captured borrow.

Mutation through a shared capture is invalid.

Copy/move behavior of the closure must preserve the borrowing rules.

## 20. Mutable reference capture

An exclusive mutable borrowed capture uses:

```sec
capture(ref mut value) fn() void {
    value += 1
}
```

The closure holds exclusive mutable authority for the lifetime of that capture.

The outer owner may not directly access the borrowed value in ways forbidden by the borrow checker while the closure retains the exclusive borrow.

A closure containing `ref mut` capture is move-only.

Its callable capability is at least `mut fn`.

If the body consumes closure environment state, the capability becomes `-> fn`.

## 21. Capture resolution

A captured name must resolve to a visible local binding in an enclosing scope.

Module-level declarations, imported declarations, types, and named functions are not local captures and may be referenced normally according to scope rules.

Undefined capture:

```sec
capture(missing) fn() void {
}
```

is invalid.

Duplicate captures are invalid.

A capture name may not conflict with a lambda parameter or another declaration where normal no-shadowing rules forbid the collision.

## 22. Capture timing

Captures are established when the lambda expression is evaluated.

Each capture source is evaluated/resolved exactly once for closure construction.

Example:

```sec
let mut value: int := 1

let first := capture(value) fn() int {
    return value
}

value = 2

let second := capture(value) fn() int {
    return value
}
```

`first()` observes its captured value `1`.

`second()` observes its captured value `2`.

A lambda expression inside a loop constructs a new callable each time execution reaches that expression.

## 23. Capture ownership commit

Capture validity is determined before the closure value becomes available.

For explicit move captures, ownership transfers into the environment when closure construction succeeds.

After successful construction, an outer binding moved into the closure may not be used.

Borrow captures begin their borrow lifetime at closure construction and remain active while the closure retains the borrow.

Closure construction must not expose a partially initialized callable value.

## 24. Escaping closures

Escaping closures are part of Sec 0.1.

A closure escapes when it may outlive the lexical scope where it was created, including when it is:

- returned from a function;
- stored in longer-lived state;
- stored in a collection with longer lifetime;
- passed to an operation that retains it.

Owned captures may escape when the closure environment can own them safely.

```sec
fn CreateOffset(offset: int) fn(int) int {
    return capture(offset) fn(value: int) int {
        return value + offset
    }
}
```

Borrowed captures may escape only while their borrow lifetime remains valid.

Invalid:

```sec
fn Invalid(value: int) fn() int {
    return capture(ref value) fn() int {
        return value
    }
}
```

The returned closure would outlive the local borrow.

No explicit lifetime syntax is introduced by lambda expressions; the borrow checker proves or rejects the lifetime.

## 25. Closure environment storage

A closure environment is compiler-managed storage.

Its physical representation is not source-observable.

The compiler may choose:

- register/SSA representation;
- complete environment elimination;
- stack storage;
- region-managed storage;
- static storage when semantically valid;
- owned dynamic storage when escaping lifetime requires it;
- another target-valid representation preserving Sec semantics.

Escaping closure support does not promise heap allocation.

If dynamic storage or another resource-bearing representation is required, that fact must remain visible to the relevant effect, escape, resource, and optimization analyses.

The source language does not expose a hidden environment pointer as a field.

## 26. Closure environment ownership

A closure owns all by-value captures.

Borrow captures remain borrows.

Destruction of a closure destroys its remaining owned environment state according to ordinary destruction rules.

A consuming `-> fn` invocation may move values out of the environment.

After a successful consuming call, the callable value is consumed and its remaining environment state is cleaned normally.

The compiler must prevent double destruction and use-after-consume.

## 27. Copy and move behavior of callable values

Named function references and non-capturing lambdas are normally copyable callable values.

A capturing closure is copyable only when its complete environment and callable capability permit safe copying.

General rules:

- all owned captures copyable + no exclusive borrow + non-consuming callable: closure may be copyable;
- shared `ref` captures may be copied only as allowed by the borrowing/reference rules;
- a move-only owned capture makes the closure move-only;
- `ref mut` capture makes the closure move-only;
- `-> fn` callable values are move-only even if their stored captures would otherwise be copyable.

Copying a copyable closure copies its owned environment by value.

For a mutable closure with copyable owned captures, each closure copy receives independent captured state.

The compiler must never implement closure copying as an untracked alias to one mutable environment unless another explicit reference-sharing abstraction defines that behavior.

## 28. Consuming closures

A closure becomes `-> fn` when invoking it consumes environment state such that the callable cannot remain valid for another invocation.

Example:

```sec
let resource := OpenResource()

let take := capture(resource) fn() Resource {
    return resource
}
```

The resulting callable is one-shot.

```sec
let value := take()

take()
```

The second call is invalid because the callable was consumed.

Consumption may arise from:

- moving an owned captured value out;
- passing an owned captured value to a consuming operation without replacement;
- another body operation that leaves the closure environment unusable under normal ownership rules.

## 29. Function-value absence and equality

A function value is not nullable.

Optional callable values use `Option`.

```sec
Option[fn(int) bool]
```

Callable values do not support semantic equality or ordering in Sec 0.1.

The physical code/environment representation is not a language-level identity contract.

## 30. Native variadic lambdas

Lambda parameters may use the same typed native variadic syntax as named functions.

```sec
let sum := fn(values: ...int) int {
    let mut total := 0

    for value in values {
        total += value
    }

    return total
}
```

All native variadic pack rules from `functions.md` apply unchanged:

- exactly one variadic parameter;
- final parameter only;
- zero arguments permitted;
- typed elements;
- read-only call-lifetime pack;
- no `Ptr`/contiguity guarantee;
- no pack escape;
- no element move-out;
- no `-> ...T`.

Lambda rules do not create separate variadic semantics.

## 31. Generic lambdas

Generic lambda parameter lists are not part of Sec 0.1.

Invalid:

```sec
fn[T](value: T) T {
    return value
}
```

A generic lambda would be an anonymous compile-time generic template rather than one concrete runtime callable value.

Use a named generic function when generic behavior is required.

```sec
fn Identity[T](value: T) T {
    return value
}
```

A concretely specialized named generic function may then be used as a callable value.

## 32. Lambda attributes

Lambda expressions do not accept attributes in Sec 0.1.

Invalid:

```sec
@inline fn(value: int) int {
    return value
}
```

Attributes remain governed by the canonical attribute rules for declaration contexts that explicitly permit them.

## 33. Direct recursion

An unnamed lambda does not see the binding being initialized by its own lambda expression.

Invalid:

```sec
let factorial := fn(value: int) int {
    return value * factorial(value - 1)
}
```

The `factorial` binding is not initialized until the lambda value has been created.

Use a named function for direct recursion.

Sec 0.1 does not define an implicit recursive-closure binding form.

## 34. Scope

A lambda body creates a function scope.

It contains:

- lambda parameters;
- explicit captures;
- local declarations.

It may access normal non-local declarations such as:

- module declarations;
- imported declarations;
- visible types;
- named functions;
- compiler-known symbols.

It may not implicitly access enclosing local bindings.

## 35. Control-flow boundary

A lambda is a function boundary.

`return` exits the lambda, not the enclosing function.

`break` and `continue` inside a lambda may not target loops outside the lambda.

Error propagation through bodyless `try` applies to the lambda's own declared return type, not to the enclosing function.

`defer` belongs to the lambda invocation scope.

## 36. Function values in data structures

Callable values may be stored in structs, unions, collections, properties, and other storage locations when:

- their callable type is valid there;
- ownership/copy/move requirements are satisfied;
- borrow lifetimes are valid;
- escaping environment requirements are satisfied.

Example:

```sec
type Handler struct {
    callback: fn(Event) void,
}
```

A move-only closure makes the containing value subject to the ordinary move-only composition rules.

## 37. Concurrency

Closure capture does not make state thread-safe.

When a callable crosses a concurrency boundary, all captured state and borrows must satisfy the relevant send/share/concurrency rules.

A shared mutable state design must use the explicit synchronization abstractions required by the concurrency model.

A `ref mut` capture does not become shareable merely because it is inside a closure.

Callable capability and copyability do not replace concurrency analysis.

## 38. Callable representation

A general callable value conceptually needs enough implementation information to invoke:

- callable code;
- optional closure environment.

No particular pair-of-pointers representation is guaranteed by Sec source semantics.

A target may represent non-capturing functions, closures, stateless callables, and other callable values differently while preserving the language contract.

ABI-visible callable representation is governed by `rules/platform/abi.md` and FFI rules.

## 39. Direct-call optimization

When the compiler proves a callable target and environment statically, it may:

- inline the body;
- eliminate the callable object;
- eliminate the environment;
- perform a direct call;
- specialize captured constants;
- stack-allocate non-escaping state;
- use another semantics-preserving optimization.

Optimization must preserve:

- capture timing;
- copy/move behavior;
- callable capability;
- destruction;
- borrow lifetime;
- side effects;
- call order.

## 40. Parser requirements

The parser must support:

- non-capturing lambda expressions;
- explicit return types;
- ordinary function parameter forms;
- consuming parameters;
- typed variadic parameters;
- capture clauses before `fn`;
- `value` captures;
- `-> value` captures;
- `ref value` captures;
- `ref mut value` captures;
- trailing capture commas;
- lambdas in expression positions.

The parser must reject or preserve for precise Sema diagnostics:

- missing parameter type;
- missing return type;
- missing body;
- malformed capture list;
- capture clause not followed by a lambda;
- arbitrary capture expressions;
- generic lambda syntax;
- lambda attributes;
- `-> ...T`;
- named function declaration syntax in an expression-only position.

## 41. AST requirements

Lambda AST must preserve at least:

```text
LambdaExpression
    captures
    parameters
    return type
    body
    source location
```

Each capture must preserve:

```text
capture name
capture mode:
    copy
    move
    shared-borrow
    mutable-borrow
source location
```

Sema must attach the inferred callable capability and resolved callable type.

The AST must not encode one fixed physical environment representation.

## 42. Sema requirements

Sema must:

- resolve every lambda parameter type;
- resolve the explicit return type;
- create a lambda function scope;
- treat owned by-value parameters as mutable locals;
- resolve explicit captures from enclosing local scopes;
- reject implicit local capture;
- reject duplicate captures;
- reject capture/parameter conflicts;
- classify capture ownership/borrow mode;
- require a copy for `capture(value)` and reject a non-copyable reusable source;
- enforce explicit `capture(<-value)` move capture;
- begin and validate borrow lifetimes for `ref`/`ref mut` captures;
- treat owned captured bindings as mutable environment state;
- infer `fn`, `mut fn`, or `-> fn` callable capability;
- ensure `ref mut` capture produces at least mutable callable capability;
- validate the body using ordinary function-body rules;
- validate return statements;
- infer the complete callable type;
- validate callable assignments and capability conversions;
- validate function-value calls;
- prevent `-> fn` reuse after consumption;
- compute closure copy/move classification;
- validate escaping closure lifetime;
- reject escaping borrowed closures when their borrows do not survive;
- preserve native variadic semantics;
- reject generic lambdas;
- treat the lambda as a boundary for return, break, continue, defer, and error propagation.

## 43. Escape and lifetime analysis

Escape analysis must distinguish at least:

- non-capturing callable;
- non-escaping closure;
- escaping owned closure;
- closure with shared borrowed capture;
- closure with mutable borrowed capture;
- consuming closure.

The analysis must determine whether the environment:

- can be eliminated;
- may remain in lexical/region storage;
- requires longer-lived owned storage;
- would outlive a borrow and must be rejected.

Storage selection is compiler-internal.

The analysis must not change source ownership semantics.

## 44. Semantic IR requirements

Semantic IR must preserve lambda/closure semantics explicitly enough to represent:

- lambda source identity;
- concrete callable signature;
- callable capability;
- ordered parameter ownership modes;
- native variadic shape;
- capture list;
- capture modes;
- concrete capture types;
- copy/move classification;
- escape classification;
- environment lifetime;
- closure construction;
- callable invocation;
- callable consumption;
- environment destruction.

IR must not prematurely lower all closures to one universal pointer pair if doing so would lose ownership, lifetime, target, or optimization information.

Non-capturing and capturing callables may converge to common lower-level representation only when legal.

## 45. Required diagnostics

The compiler must diagnose at least:

```text
lambda return type is required
```

```text
duplicate parameter value
```

```text
lambda must return int
```

```text
lambda must return int, got bool
```

```text
lambda cannot access outer variable factor without explicit capture
```

```text
undefined capture factor
```

```text
duplicate capture factor
```

```text
capture factor conflicts with lambda parameter factor
```

```text
cannot capture non-movable value resource
```

```text
use of moved value resource
```

```text
borrowed capture value does not live long enough
```

```text
closure with mutable borrowed capture is move-only
```

```text
callable requires mutable access
```

```text
callable is consumed by this call
```

```text
use of consumed callable
```

```text
generic lambdas are not supported in Sec 0.1
```

```text
lambda attributes are not supported in Sec 0.1
```

```text
lambda cannot directly reference the binding being initialized
```

Diagnostics should identify the lambda, capture, callable value, or source binding responsible for the violation.

## 46. Best practice

- Prefer named functions when behavior has a stable reusable name or needs generic parameters.
- Use lambdas for local behavior and higher-order operations.
- Keep captures explicit and minimal.
- Prefer owned captures when a closure must escape.
- Use `ref` capture only when shared borrowing is semantically intended.
- Use `ref mut` capture sparingly because it reserves exclusive authority for the closure lifetime.
- Let callable capability be inferred from the body instead of annotating lambda expressions.
- Avoid depending on closure allocation strategy or representation.
- Use a consuming closure only for genuinely one-shot behavior.
- Keep foreign callbacks behind ABI/FFI rules rather than assuming native closure representation is foreign-compatible.

## 47. Cross-rulebook ownership

This rulebook owns lambda expressions, explicit captures, closure environments, callable capability inference, callable value copy/move composition, and closure escape semantics.

Related rules are owned elsewhere:

- ordinary parameters, `->` parameters, native variadics, return values, call order, and call transfer commit: `rules/declarations/functions.md`;
- generic named functions and specialization: `rules/declarations/generics.md`;
- interface callable receiver capability terminology: `rules/declarations/interfaces.md`;
- ownership, move, copy, destruction, and use-after-move: memory rulebooks;
- `ref` and `ref mut` borrowing/lifetimes: borrowing rulebook;
- error propagation and `try`: error-handling rulebook;
- concurrency crossing and synchronization: concurrency rulebooks;
- effects and escape analysis: compiler analysis rulebooks;
- physical callable ABI and foreign callbacks: `rules/platform/abi.md` and FFI rulebook.
