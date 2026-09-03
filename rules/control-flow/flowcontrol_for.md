# `for` Loop

- **Status:** Normative
- **Created:** 2026-08-16
- **Last updated:** 2026-09-03
- **Document revision:** 2.1
- **Sec language version:** 0.1
- **Canonical path:** `rules/control-flow/flowcontrol_for.md`
- **Replaces:** `rules/control-flow/flowcontrol_for.txt`
- **Repository baseline reviewed:** `998d8d1`

---

## 1. Purpose

`for` is Sec's iteration statement.

It is used for:

- infinite loops;
- finite numeric ranges;
- compiler-known sequential collections;
- maps;
- sets;
- strings;
- other compiler-known iterable categories explicitly defined by the language.

Condition-controlled repetition belongs to `while`.

Sec 0.1 does not support:

- C-style `for` loops;
- condition-only `for` loops;
- implicit consuming iteration;
- iterator discovery by naming convention.

---

## 2. Core forms

Infinite loop:

```sec
for {
    Work()
}
```

Single-binding iteration:

```sec
for item in items {
    Process(item)
}
```

Sequential index and element iteration:

```sec
for index, item in items {
    Process(index, item)
}
```

Map iteration:

```sec
for key, value in users {
    Process(key, value)
}
```

Range iteration:

```sec
for index in 0..<10 {
    Process(index)
}
```

Explicit descending range:

```sec
for index in 10..1 step -1 {
    Process(index)
}
```

Shared element iteration:

```sec
for ref item in items {
    Inspect(item)
}
```

Mutable element iteration:

```sec
for ref mut item in items {
    item.Reset()
}
```

Braces are mandatory for every `for` body.

---

## 3. Separation from `while`

Sec intentionally separates iteration from condition-controlled repetition.

Use:

```sec
for item in items {
}
```

for iteration.

Use:

```sec
while ready {
}
```

for repetition controlled by a `bool` condition.

This is invalid:

```sec
for ready {
}
```

This is also invalid:

```sec
for i := 0; i < 10; i += 1 {
}
```

---

## 4. Loop bindings

A loop binding is declared by the `for` statement and exists only inside the loop body.

Supported binding modes are:

```text
item
ref item
ref mut item
_
```

The source iterable determines which modes are valid for each binding position.

A plain binding is a by-value binding.

A `ref` binding is a shared borrowed binding.

A `ref mut` binding is an exclusive mutable borrowed binding.

`_` discards that binding position and does not introduce a symbol.

There is no consuming binding mode in Sec 0.1.

In particular, Sec 0.1 does not define:

```sec
for -> item in items {
}
```

or:

```sec
for item in move items {
}
```

---

## 5. Plain by-value iteration

A plain loop binding receives a value of the yielded element type.

Example:

```sec
for item in items {
    Process(item)
}
```

If the yielded element type `T` is implicitly copyable, each iteration copies the current element into `item`.

The source element remains valid.

If `T` is move-only, the plain binding is invalid.

The compiler must not silently reinterpret the loop as borrowing and must not silently move the element out of the source collection.

Example:

```sec
for file in files {
    Use(file)
}
```

is invalid when `File` is move-only.

Use an explicit shared borrow when the value only needs to be inspected:

```sec
for ref file in files {
    Inspect(file)
}
```

Use a collection-specific owning extraction operation when ownership must be removed from the collection.

Ordinary iteration is never an implicit extraction operation.

---

## 6. Shared element iteration

Shared iteration is written:

```sec
for ref item in items {
    Inspect(item)
}
```

For an element type `T`, the loop binding has type:

```text
ref T
```

The source must be able to provide a stable shared reference to the current element.

Shared iteration is valid for move-only element types because no copy or ownership transfer occurs.

The normal borrowing rules apply.

The loop must preserve the validity of the current element reference and of the iteration state.

A shared element reference must not be used to move the element out of the source.

---

## 7. Mutable element iteration

Mutable element iteration is written:

```sec
for ref mut item in items {
    item.Reset()
}
```

For an element type `T`, the loop binding has type:

```text
ref mut T
```

The source must provide mutable borrowing authority for the element.

The normal exclusive-borrow rules apply.

The loop may mutate the current element through the `ref mut` binding.

Example:

```sec
for ref mut item in items {
    item.Count += 1
}
```

Mutable element access does not grant permission to structurally mutate the iterated collection.

This is invalid when `Append` may change collection structure or backing storage:

```sec
for ref mut item in items {
    try items.Append(newItem)
}
```

---

## 8. Structural stability during iteration

An active collection iteration must preserve the structural assumptions used by the iterator.

Structural mutation includes operations that may:

- change collection length;
- insert or remove elements;
- move or replace backing storage;
- reorder elements in a way that invalidates iteration state;
- invalidate current or future element references.

Such mutation is invalid while the iteration depends on the current structure.

Element mutation through an approved `ref mut` loop binding is not structural mutation when it preserves the collection's structural invariants.

Examples of structural operations include, where applicable:

```text
Append
Clear
RemoveAt
Remove
```

The exact operation set is owned by the collection rulebook.

---

## 9. Discard bindings

`_` may be used in a supported binding position.

Examples:

```sec
for _, item in items {
    Process(item)
}
```

```sec
for _, ref mut value in users {
    Update(value)
}
```

A discard does not create a local symbol.

Discarding a binding position does not require the compiler to copy or move the yielded value merely to discard it.

This is important for move-only values and keys.

The iterable still advances normally.

---

## 10. Loop-binding immutability

A plain loop binding is an immutable local binding.

This is invalid:

```sec
for item in items {
    item = replacement
}
```

A `ref` binding is an immutable binding containing shared reference authority.

A `ref mut` binding is an immutable binding containing exclusive mutable reference authority.

The binding itself is not reassigned merely because the referenced element is mutable.

Example:

```sec
for ref mut item in items {
    item.Count += 1
}
```

is valid when the element mutation is valid.

---

## 11. Scope and shadowing

Every loop body is a lexical scope.

Loop bindings exist only inside that body.

This is invalid:

```sec
for item in items {
    Process(item)
}

Process(item)
```

Normal Sec no-shadowing rules apply.

Example:

```sec
let item := 10

for item in items {
}
```

is invalid because the loop binding would shadow an existing visible symbol.

Nested loops follow the same rule.

---

## 12. Iterable expression evaluation

The iterable expression is evaluated exactly once before iteration begins.

Example:

```sec
for item in LoadItems() {
    Process(item)
}
```

`LoadItems()` is evaluated once.

It is not reevaluated for every element.

Any owned temporary created as the iterable source remains valid for as long as required by the loop and is destroyed according to the normal ownership and cleanup rules.

---

## 13. Sec 0.1 iterable categories

Sec 0.1 `for` iteration is language-defined for approved compiler-known categories.

The canonical categories include:

- fixed arrays `T[N]`;
- owning dynamic arrays `T[]`;
- slices `ref T[]` and `ref mut T[]`;
- `list[T]`;
- `set[T]`;
- `map[K, V]`;
- `string`;
- finite ranges;
- `vector[T, N]` where the shaped-type rules define sequential iteration.
- concrete types with explicit compiler-known `Iterator[T]` conformance.

A type is not iterable merely because it has members such as:

```text
Len
Get
Next
HasNext
Contains
```

An ordinary member named `Next` does not establish iteration. Section 37 defines
the sole user-type participation point: explicit conformance to the
compiler-known `Iterator[T]` interface.

---

## 14. Sequential collections

Sequential collections support one element binding:

```sec
for item in items {
}
```

and, where defined, two index/element bindings:

```sec
for index, item in items {
}
```

The first binding is the zero-based sequential iteration index.

Its type is:

```text
int
```

The first sequential index binding may be a normal value binding or `_`.

It is not an element reference and therefore does not use `ref` or `ref mut`.

The second binding follows the element binding rules.

Examples:

```sec
for index, ref item in items {
    Inspect(index, item)
}
```

```sec
for index, ref mut item in items {
    item.Sequence = index
}
```

---

## 15. Fixed arrays

For a fixed array:

```text
T[N]
```

iteration proceeds from index `0` through `N - 1`.

Plain element iteration copies `T` and therefore requires `T` to be implicitly copyable.

Shared iteration may borrow each element:

```sec
for ref item in values {
}
```

Mutable iteration may borrow each element mutably when the source has mutable authority:

```sec
for ref mut item in values {
}
```

A `for` loop does not partially move elements out of a fixed array.

---

## 16. Dynamic arrays and lists

For `T[]` and `list[T]`, iteration visits the live logical elements in collection order.

Plain iteration copies each yielded `T` and requires `T` to be implicitly copyable.

Shared iteration yields `ref T`.

Mutable iteration yields `ref mut T` when the source provides mutable authority.

Structural mutation of the same collection is invalid while iteration is active when that mutation may invalidate iteration state.

The loop does not expose or depend on collection capacity as a source-language concept.

---

## 17. Slices

A slice is already borrowed storage.

For:

```text
ref T[]
```

valid element modes are:

- plain value, when `T` is copyable;
- shared `ref T`;
- discard `_`.

A shared slice does not grant mutable element authority.

For:

```text
ref mut T[]
```

valid element modes include:

- plain value, when `T` is copyable;
- shared `ref T`;
- mutable `ref mut T`;
- discard `_`.

Iteration does not extend the lifetime of the backing owner beyond normal reference rules.

---

## 18. Vectors

Where `vector[T, N]` supports sequential iteration, it follows the sequential collection binding model.

The iteration order is the vector's logical element order.

Plain element binding requires copyable `T`.

Shared and mutable element bindings follow the shaped-type and borrowing rules for the concrete vector value or reference.

No additional public iterator object is implied.

---

## 19. Strings

String iteration yields Unicode code points.

The yielded element type is:

```text
rune
```

Example:

```sec
for character in text {
    Process(character)
}
```

Two-binding string iteration may provide the zero-based sequential iteration index and the yielded `rune`:

```sec
for index, character in text {
    Process(index, character)
}
```

This iteration index is an iteration index and does not imply exposure of the string's encoded byte representation.

String iteration does not yield raw UTF-8 bytes.

Use an explicit byte representation when encoded bytes are required.

Because the yielded rune is a decoded value rather than a mutable stored rune element, `ref` and `ref mut` rune bindings are not provided by ordinary string iteration.

---

## 20. Sets

A set supports exactly one logical value binding or discard.

Examples:

```sec
for value in activeUsers {
    Process(value)
}
```

```sec
for ref value in activeUsers {
    Inspect(value)
}
```

Plain set iteration copies the stored value and requires the element type to be implicitly copyable.

Shared reference iteration is valid when the set can provide a stable shared element reference.

Mutable element iteration is invalid:

```sec
for ref mut value in activeUsers {
}
```

A stored set value participates in equality and hashing identity.

Mutating it in place could invalidate the set's lookup invariants.

Use explicit set operations to replace or remove values.

Set iteration order is unspecified unless a specific ordered set type explicitly defines an order.

Programs must not depend on the iteration order of ordinary `set[T]`.

---

## 21. Maps

A map requires exactly two logical binding positions:

```sec
for key, value in users {
    Process(key, value)
}
```

Single-binding map iteration is not part of Sec 0.1:

```sec
for entry in users {
}
```

The two positions are always:

```text
key, value
```

Plain key/value bindings copy their respective values and therefore require the copied type to be implicitly copyable.

Shared borrowing is permitted:

```sec
for ref key, ref value in users {
    Inspect(key, value)
}
```

Mutable value borrowing is permitted when the map source provides mutable authority:

```sec
for ref key, ref mut value in users {
    Update(value)
}
```

A map key must not be mutably borrowed through iteration:

```sec
for ref mut key, value in users {
}
```

is invalid.

A stored key participates in hash/equality identity and must not be changed in place while stored in the map.

Discard forms are valid:

```sec
for _, ref mut value in users {
    Update(value)
}
```

```sec
for ref key, _ in users {
    InspectKey(key)
}
```

Map iteration order is unspecified unless a distinct ordered map type explicitly defines an order.

Programs must not depend on the iteration order of ordinary `map[K, V]`.

---

## 22. Range forms

Finite ranges are directly iterable.

Supported finite forms are:

```text
start..end
start..<end
```

`..` includes the end value when it is reached by the progression.

`..<` excludes the end value.

Open-ended ranges are not directly iterable by `for`:

```text
start..
..end
..<end
```

Example of an invalid loop:

```sec
for value in 0.. {
}
```

Use `for {}` or `while` when repetition has no finite range boundary.

---

## 23. Range bound evaluation

The start expression is evaluated exactly once.

The end expression is evaluated exactly once.

If present, the `step` expression is evaluated exactly once.

Example:

```sec
for value in Start()..<End() step Increment() {
    Process(value)
}
```

`Start()`, `End()`, and `Increment()` are each evaluated once before range progression begins.

---

## 24. Range type compatibility

Range bounds must have compatible ordered numeric types according to the normal Sec type rules.

Named numeric types retain their identity.

Example:

```sec
let minimum: Score := Score(0)
let maximum: Score := Score(10)

for score in minimum..maximum {
    Process(score)
}
```

The loop binding has type `Score`.

The compiler must not erase named-type identity merely to construct a range.

An explicit step must be valid for the range's numeric domain and progression operation.

---

## 25. Implicit range step

When `step` is omitted, integer-like range iteration always uses an ascending progression of `+1`.

The compiler does not infer descending iteration from the relationship between the bounds.

Example:

```sec
for value in 1..5 {
}
```

iterates:

```text
1
2
3
4
5
```

Example:

```sec
for value in 1..<5 {
}
```

iterates:

```text
1
2
3
4
```

Example:

```sec
for value in 10..<5 {
}
```

performs zero iterations.

There is no implicit `-1` step.

---

## 26. Explicit range step

An explicit step is written:

```sec
for value in start..<end step increment {
    Process(value)
}
```

The step:

- is evaluated exactly once;
- must be non-zero;
- must be valid for the range type;
- determines the direction and delta of the progression.

A positive step is ascending.

A negative step is descending.

Descending iteration must therefore be explicit.

Example:

```sec
for value in 10..1 step -1 {
    Process(value)
}
```

For compile-time known bounds and step, a step whose sign cannot progress toward the requested bound is invalid.

Examples:

```sec
for value in 1..10 step -1 {
}
```

```sec
for value in 10..1 step 1 {
}
```

are invalid when those values are compile-time known.

A zero step is invalid.

The compiler must never silently convert an explicit step to another sign.

---

## 27. Unsigned descending ranges

Descending range iteration requires a progression domain that can represent the explicit negative step.

The compiler must not invent an implicit signed conversion for an unsigned range.

If the ordinary type rules do not permit a negative step for the selected range type, use a signed range type or express the operation with `while`.

This keeps signedness explicit.

---

## 28. Float and decimal ranges

`float` and `decimal` range iteration requires an explicit step.

This is valid:

```sec
for value in 0.001..<0.002 step 0.00001 {
    Process(value)
}
```

This is also a valid descending form:

```sec
for value in 0.002..0.001 step -0.00001 {
    Process(value)
}
```

This is invalid because the step is omitted:

```sec
for value in 0.001..<0.002 {
}
```

The progression must use the arithmetic semantics of the resolved numeric type.

Float lowering must avoid changing source-visible range coverage merely because of avoidable cumulative repeated-addition drift.

Decimal progression must preserve Sec decimal arithmetic semantics.

---

## 29. Range termination

A range loop stops when the next logical value would be outside the selected inclusive or exclusive boundary in the progression direction.

The implementation must not require an overflowing increment after the final yielded value.

Inclusive ascending iteration stops after yielding `end` when `end` is reached.

Exclusive ascending iteration stops before yielding `end`.

Inclusive descending iteration stops after yielding `end` when `end` is reached.

Exclusive descending iteration stops before yielding `end`.

If the progression cannot yield any valid value, the loop performs zero iterations.

---

## 30. `break`

`break` exits the nearest enclosing loop.

Example:

```sec
for item in items {
    if Stop(item) {
        break
    }
}
```

`break` is invalid outside a loop.

`switch` does not create a separate `break` target in Sec.

Therefore a `break` inside a `switch` nested in a loop exits the nearest enclosing loop.

Cleanup required by the exited scopes runs before control reaches the loop exit.

Labeled `break` is not part of Sec 0.1.

---

## 31. `continue`

`continue` advances the nearest enclosing loop to its next iteration.

Example:

```sec
for item in items {
    if Skip(item) {
        continue
    }

    Process(item)
}
```

`continue` is invalid outside a loop.

Per-iteration cleanup required by scopes exited through `continue` runs before the next iteration begins.

Labeled `continue` is not part of Sec 0.1.

---

## 32. Return and error propagation

`return` exits the enclosing function normally.

A bodyless propagating `try` may also leave the loop through the function's error-return path.

Both paths perform all required cleanup according to the ownership, destruction, and `defer` rules.

A loop does not suppress or alter Result propagation semantics.

---

## 33. Definite assignment after finite loops

A finite `for` loop may execute zero times.

Therefore assignment performed only inside a finite loop does not by itself establish definite assignment after the loop.

Conceptually:

```sec
let mut result: int

for value in values {
    result = value
}

Use(result)
```

is invalid unless another rule proves `result` is initialized on every path reaching `Use`.

The compiler must preserve the zero-iteration path in control-flow analysis.

---

## 34. Return-path analysis

A `return` inside an ordinary finite `for` loop does not by itself prove that a non-void function returns on every path because the loop may execute zero times.

An infinite loop:

```sec
for {
}
```

has no normal fallthrough path unless a reachable `break` can exit it.

An infinite loop with no reachable `break` is non-continuing for return-path analysis.

If a reachable `break` exists, the post-loop path must be analyzed normally.

---

## 35. Ownership and cleanup per iteration

Each iteration creates its loop-body lexical scope.

Owned values created during one iteration are destroyed before the iteration completes unless ownership is transferred elsewhere.

`continue` performs required iteration cleanup.

`break` performs cleanup for scopes exited by the break.

`return` and error propagation perform function-exit cleanup.

Loop-provided references do not create ownership of source elements.

Plain by-value copies create ordinary local values with the normal destruction semantics of their type.

---

## 36. No consuming `for` in Sec 0.1

Sec 0.1 intentionally does not define a consuming `for` form.

The language does not provide:

```sec
for -> item in items {
}
```

and does not reinterpret:

```sec
for item in items {
}
```

as a move when `T` is move-only.

Consuming iteration would require additional rules for:

- partially consumed collections;
- remaining-element destruction after `break`;
- error and return exits;
- fixed-array holes or element initialization state;
- map and set extraction invariants;
- collection backing reclamation.

Those semantics are deliberately not part of Sec 0.1.

When ownership of stored elements must be extracted, use the explicit owning operation defined by the collection type.

---

## 37. Compiler-known `Iterator[T]`

Sec 0.1 defines one compiler-known generic interface for stateful, pull-based
iteration:

```sec
interface Iterator[T] {
    mut fn Next() Option[T]
}
```

A concrete type becomes a `for` source only through explicit conformance on its
primary implementation:

```sec
impl NumberStream implements Iterator[int] {
    fn Next() Option[int] {
        // Return Some(value), or None when iteration is complete.
    }
}
```

The compiler must not decide that a type is iterable merely because it exposes names such as:

```text
Next
HasNext
Get
Len
Iterator
```

The following rules apply:

- `for value in source` repeatedly invokes the statically resolved concrete
  `Next()` implementation;
- `Some(value)` initializes the single loop binding and executes the body;
- `None` terminates the loop without executing the body again;
- the source expression is evaluated exactly once;
- reusable iterator storage must provide mutable authority, while a fresh owned
  temporary is retained as compiler-generated local state for the loop;
- iterator loops have exactly one value binding or discard binding;
- the returned `T` is an owned yielded value, not an implicit reference into
  iterator storage;
- `ref` and `ref mut` iterator-result bindings are not defined by this protocol;
- conformance and the concrete `Next` call are resolved at compile time;
- the protocol introduces no interface object, virtual dispatch, boxing,
  allocation, runtime type inspection, or runtime borrow state.

`Iterator[T]` does not define a separate `Iterable[T]` factory protocol. A
collection may expose an ordinary method that returns a concrete iterator, and
that returned value may be used directly:

```sec
for part in text.Split(";") {
    Process(part)
}
```

Lowering must consume Sema's resolved iterator operation. A backend must not
rediscover the protocol from the spelling `Next`.

---

## 38. No hidden allocation

Creating or executing a `for` loop must not inherently require heap allocation.

Compiler-generated iteration state should use ordinary local or lowered state when possible.

A collection operation performed by the loop body may allocate according to that operation's own rules, but iteration itself does not gain permission for hidden allocation.

---

## 39. Evaluation and observable order

The iterable expression is evaluated before iteration begins.

For ranges, the start, end, and explicit step expressions are each evaluated once before progression begins.

Within each iteration, the loop body follows normal Sec statement and expression evaluation order.

The compiler may optimize loop machinery only when it preserves:

- yielded values;
- iteration order where order is defined;
- ownership and borrow semantics;
- cleanup and destruction order;
- volatile effects;
- error propagation;
- `break` and `continue` behavior.

---

## 40. Unsupported forms

Sec 0.1 does not support:

```text
condition-only for
C-style for
loop else clauses
labeled break or continue
implicit consuming iteration
for -> item in source
for item in move source
implicit mutable element iteration
ref binding to decoded string runes
ref binding to synthesized range values
mutable set element iteration
mutable map key iteration
single-binding map iteration
general pattern destructuring in for headers
iterator discovery by member names
implicit Iterable factory discovery
runtime-dispatched Iterator interface values
```

A future rule may add a feature from this list only by explicitly defining its syntax, ownership, borrowing, cleanup, and lowering behavior.

---

## 41. Diagnostic requirements

Diagnostics should distinguish at least:

- non-iterable source;
- invalid number of bindings;
- `Iterator[T]` use without explicit conformance;
- iterator source without mutable authority;
- iterator `Next` signature incompatible with `Option[T]`;
- invalid binding mode for the iterable category;
- plain by-value iteration of a move-only element;
- mutable iteration without mutable authority;
- mutable map key iteration;
- mutable set element iteration;
- structural mutation during active iteration;
- invalid open-ended range iteration;
- incompatible range bound types;
- invalid or zero explicit step;
- invalid step direction when statically known;
- missing explicit step for float or decimal ranges;
- `break` outside a loop;
- `continue` outside a loop;
- loop binding shadowing;
- reassignment of a loop binding.

Diagnostics should identify both the loop source and the operation or binding that violates the rule when practical.

---

## 42. Compiler obligations

Before lowering, the compiler must resolve:

- iterable category;
- binding count and meaning;
- yielded type for every binding position;
- plain copy versus shared borrow versus mutable borrow;
- source mutability and borrow authority;
- structural-stability requirements;
- range type and direction;
- explicit or implicit range step;
- loop-body scope;
- `break` and `continue` targets;
- ownership and cleanup edges;
- zero-iteration flow where possible.

Semantic IR must not leave ownership mode ambiguous for loop bindings.

The backend must not infer source-language copy, move, or borrow behavior from physical loads and stores.

---

## 43. Relationship to other rulebooks

This rulebook owns `for` source semantics.

Related rules remain authoritative for their own domains:

- `rules/collections/collections.md` for collection storage and mutation invariants;
- `rules/memory/ownership.md` for ownership;
- `rules/memory/copy_move.md` for copy and move classification;
- `rules/memory/borrowing.md` for borrow validity;
- `rules/memory/destruction.md` for cleanup;
- `rules/control-flow/defer.md` for deferred cleanup;
- `rules/foundations/grammar.md` for canonical grammar;
- shaped-type rulebooks for `vector` and other shaped collection behavior.

When another rulebook defines that a particular source cannot provide a requested reference or mutable authority, `for` does not override that restriction.

---

## 44. Summary

Sec 0.1 `for` follows these central rules:

```text
for item in items
    by-value copy
    requires copyable yielded type
    never implicitly moves or borrows

for ref item in items
    shared element borrow

for ref mut item in items
    exclusive mutable element borrow
    only where the iterable permits it

ordinary iteration
    never consuming

range without step
    integer-like only
    always ascending +1
    start > end means zero iterations

range with explicit negative step
    descending

float / decimal range
    explicit step required

map
    exactly key, value bindings
    key cannot be ref mut

set
    one value binding
    value cannot be ref mut

explicit implements Iterator[T]
    one owned T binding; repeated statically resolved Next() calls until None

member named Next without conformance
    not iterable
```
