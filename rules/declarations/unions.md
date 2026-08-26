# Unions

## Document metadata

- **Status:** Normative
- **Created:** 2026-08-13
- **Last updated:** 2026-08-26
- **Document revision:** 2
- **Replaces:** unions.txt
- **Sec language version:** 0.1
- **Canonical path:** `rules/declarations/unions.md`

---

## 1. Purpose

A Sec union is a closed nominal type whose runtime value contains exactly one active declared variant.

A variant may:

- carry no payload;
- carry one unnamed payload value; or
- carry a struct-like payload with named fields.

Unions are used when one value may be one of several explicitly declared alternatives and variant-specific data must remain type-safe.

Example:

```sec
type Message union {
    Quit
    Text(string)
    Move {
        x: int
        y: int
    }
}
```

A union is distinct from:

- an enum, which represents one named value from a finite value domain;
- a struct, whose declared fields exist together;
- an interface, which describes behavior;
- `null`, which is not the union model;
- `Option[T]`, whose `None` case is an actual value;
- the compiler-known `empty` initialization state described in this rulebook.

---

## 2. Core semantic model

A union value has:

1. one active declared variant; and
2. the payload required by that variant, if any.

The following rules are normative:

- A union is declared with `type Name union`.
- A union must be named.
- A union must declare at least one variant.
- Union types are nominal.
- Union variants belong to the union namespace.
- Variants are constructors, not standalone runtime types.
- Variant names must be unique within one union.
- Different unions may reuse variant names.
- Duplicate payload types are allowed.
- Unions are closed: variants cannot be added outside the original declaration.
- A union may be generic.
- A union may be nested where nested type declarations are otherwise permitted.
- Direct recursive storage is invalid.
- Recursion through a finite indirection is valid.
- No implicit conversion exists between a union and one of its payload types.
- No implicit conversion exists between distinct union types.
- The active variant is not exposed as a source-level integer tag.

---

## 3. Syntax

A tagged union may be marked as a concrete Sec error type:

```sec
type IOError union error {
    OpenError {
        Path: string
        Code: int
    }
}
```

The `error` marker makes the complete union assignable to compiler-known
`error` without changing variant, layout, ownership, borrowing, destruction,
default, or match semantics. Concrete identity, active variant, and payload
remain available after widening. The concrete union is closed even though the
root `error` domain is open.

### 3.1 Payload-less variants

```sec
type State union {
    Idle
    Running
    Stopped
}
```

Construction:

```sec
let state: State := State.Idle
```

### 3.2 Single unnamed payload

```sec
type Number union {
    Integer(int)
    Decimal(decimal)
}
```

Construction:

```sec
let number: Number := Number.Integer(42)
```

A single-payload variant carries exactly one payload value.

Multiple unnamed payload values are not supported. Use a struct-like payload when several named values belong to one variant.

### 3.3 Struct-like payload

```sec
type Shape union {
    Circle {
        radius: decimal
    }

    Rectangle {
        width: decimal
        height: decimal
    }
}
```

Construction:

```sec
let shape: Shape := Shape.Rectangle {
    width: 10
    height: 20
}
```

A struct-like payload is part of the variant. It is not a separate nominal source-level type.

### 3.4 Mixed unions

Variant forms may be mixed within one union:

```sec
type Message union {
    Quit
    Text(string)
    Move {
        x: int
        y: int
    }
}
```

---

## 4. Variant namespaces

For a module-level union:

```sec
type State union {
    Idle
    Running
}
```

construction uses the union namespace:

```sec
State.Idle
State.Running
```

An unqualified variant name is not introduced as an ordinary module symbol merely because the union declares it.

Pattern contexts may resolve an unqualified variant name from the known union subject type:

```sec
match state {
    Idle => HandleIdle()
    Running => HandleRunning()
}
```

The fully qualified form remains valid where qualification is required for disambiguation.

---

## 5. Construction

### 5.1 Payload-less variants

A payload-less variant accepts no payload arguments.

```sec
let state := State.Idle
```

Invalid:

```sec
let state := State.Idle(1)
```

### 5.2 Single-payload variants

A single-payload variant requires exactly one value assignable to its declared payload type.

```sec
let number := Number.Integer(42)
```

Invalid:

```sec
let number := Number.Integer()
```

Invalid:

```sec
let number := Number.Integer(1, 2)
```

### 5.3 Struct-like payload construction

Struct-like variants use named-field construction.

```sec
let rectangle := Shape.Rectangle {
    width: 10
    height: 20
}
```

Positional construction is invalid:

```sec
let rectangle := Shape.Rectangle(10, 20)
```

### 5.4 Omitted struct-like payload fields

A struct-like payload field may be omitted when its declared type is `Defaultable`.

The omitted field receives the default value of its declared type.

Example:

```sec
type MoveCommand union {
    Move {
        x: int
        y: int
    }
}

let command := MoveCommand.Move {
    x: 10
}
```

The resulting payload has:

```text
x = 10
y = 0
```

A field whose type is `NonDefaultable` must be supplied explicitly.

Unknown fields and duplicate fields are compile-time errors.

Field values must be assignable to their declared field types.

---

## 6. Default variants and defaultability

### 6.1 No implicit first-variant default

A union does not automatically use its first declared variant as a default.

Declaration order alone never selects a union default.

This differs intentionally from enum default semantics.

### 6.2 Explicit default variant

At most one union variant may be marked `default`.

Payload-less form:

```sec
type State union {
    Idle default
    Running
}
```

Single-payload form:

```sec
type Value union {
    Number(int) default
    Text(string)
}
```

Struct-like form:

```sec
type Position union {
    Unknown

    Known {
        x: int
        y: int
    } default
}
```

The `default` marker belongs to the variant declaration.

### 6.3 Defaultability of the selected variant

A payload-less default variant is always constructible.

A single-payload default variant is default-constructible only when its payload type is `Defaultable`.

A struct-like default variant is default-constructible only when every field that must be initialized can be resolved through the normal struct-like payload default rules.

If the marked default variant cannot be default-constructed, the union declaration is invalid.

### 6.4 Union defaultability

A union is `Defaultable` exactly when it declares one valid explicit default variant.

Otherwise the union is `NonDefaultable`.

Example:

```sec
type State union {
    Idle default
    Running
}

let mut state: State
```

is equivalent in semantic value to:

```sec
let mut state: State := State.Idle
```

An immutable binding must still be initialized explicitly:

```sec
let state: State
```

is invalid even when `State` is `Defaultable`.

---

## 7. The `empty` initialization state

### 7.1 Purpose

A mutable union binding whose union type is `NonDefaultable` may omit its initializer as a union-specific definite-assignment exception:

```sec
type State union {
    Idle
    Running
}

let mut state: State
```

Immediately after this declaration, `state` is `empty`.

### 7.2 `empty` is not a union value

`empty` means that the storage for the mutable union binding does not currently contain a constructed union value.

It is compiler-known initialization state.

`empty` is not:

- a hidden union variant;
- part of the union's closed value domain;
- `null`;
- `None`;
- `Nothing`;
- an integer tag exposed to the program.

The compiler must not model the source type as if every union declaration implicitly contained an `Empty` variant.

### 7.3 Empty state is local initialization state

An empty union binding may not be passed as a value, returned, copied, moved, borrowed, stored into another value, or otherwise exposed where an actual value of the union type is required.

Before such use, one declared variant must be assigned.

Example:

```sec
let mut state: State
state = State.Idle
UseState(state)
```

### 7.4 Assignment activates a variant

Assignment of a valid union value transitions an empty binding to an initialized union value:

```sec
let mut state: State
state = State.Running
```

After the assignment, `state` has active variant `Running`.

### 7.5 Destruction of empty storage

Destroying storage that is still empty performs no union payload destruction because no union value was constructed there.

### 7.6 Moved-from is not empty

A moved-from union binding follows the normal ownership rules and is unusable.

It does not become an observable `empty` state.

```sec
let mut source := State.Idle
let target :<- source

// Invalid: moved-from storage is not an empty union state.
if source is empty {
    ...
}
```

### 7.7 Runtime-dependent initialization

Control flow may make the initialization state runtime-dependent:

```sec
let mut state: State

if condition {
    state = State.Idle
}
```

If later code observes `empty`, the compiler may carry an internal initialization flag, SSA state, or equivalent representation required to preserve the semantics.

### 7.8 Payload ownership transfer

Union construction never silently consumes a reusable source place. A
non-copyable reusable payload requires an explicit move, for example
`Choice.Some(<-resource)` or `Payload: <-resource` in a struct-like variant.
Plain payload syntax may copy a copyable source; fresh temporaries need no move
marker. A moved-from union is unavailable under the ownership rules and is not
the union-specific `empty` state.

Such implementation state is not part of the source-level union value or its declared variant set.

---

## 8. `is` tests for union state and active variant

### 8.1 Active variant test

`is` tests whether a union value currently has one specified active variant without destructuring or binding its payload.

```sec
if state is Idle {
    HandleIdle()
}
```

The test is valid for payload-carrying variants as well:

```sec
if result is Ok {
    HandleSuccessState()
}
```

No payload variable is introduced by `is`.

Use `match` when payload extraction is required.

### 8.2 Empty-state test

A mutable union binding whose control-flow state may be empty may be tested with:

```sec
if state is empty {
    state = State.Idle
}
```

`is empty` observes initialization state, not a union variant.

### 8.3 Data-flow refinement

The compiler must use `is` tests for definite-assignment and reachability refinement where possible.

Example:

```sec
let mut state: State

if state is empty {
    state = State.Idle
}

UseState(state)
```

After the `if`, the compiler can prove that `state` is initialized:

- the true branch assigns a value;
- the false branch means the binding was already non-empty.

Likewise, within:

```sec
if state is Running {
    ...
}
```

the compiler knows that `state` is initialized and its active variant is `Running`.

### 8.4 Impossible state tests

A value context that guarantees an actual initialized union value cannot be empty.

Example:

```sec
fn Process(state: State) void {
    if state is empty {
        ...
    }
}
```

The compiler should diagnose an impossible or meaningless `is empty` test according to the normal proven-impossible-code diagnostic policy.

---

## 9. `match`

`match` is the primary operation for exhaustive union inspection and payload binding.

### 9.1 Payload-less variants

```sec
match state {
    Idle => HandleIdle()
    Running => HandleRunning()
    Stopped => HandleStopped()
}
```

### 9.2 Single-payload variants

```sec
match number {
    Integer(value) => UseInt(value)
    Decimal(value) => UseDecimal(value)
}
```

### 9.3 Whole struct-like payload binding

The complete struct-like payload may be bound as one value:

```sec
match shape {
    Circle(circle) => circle.radius
    Rectangle(rectangle) => rectangle.width * rectangle.height
}
```

This form remains canonical and supported.

### 9.4 Shallow named-field destructuring

A struct-like union payload may destructure its named fields directly:

```sec
match shape {
    Circle { radius } => radius * radius
    Rectangle { width, height } => width * height
}
```

Field destructuring is shallow.

It destructures only the named fields of the selected union variant payload.

A bound field that itself contains structured or variant data is handled by ordinary field access or another `match`.

### 9.5 Partial field binding

A field-destructuring pattern need not mention every field.

```sec
match shape {
    Rectangle { width } => Use(width)
    _ => Other()
}
```

Omitted fields are simply not bound.

They do not receive defaults, and omission does not add a value constraint to the pattern.

`Rectangle { width }` still covers every `Rectangle` value unless a guard adds an additional condition.

No `..` marker is required merely to ignore unbound fields.

### 9.6 Binding rename

A destructured field may be bound under a different local name:

```sec
match shape {
    Rectangle {
        width: w
        height: h
    } => w * h

    _ => 0
}
```

The shorthand:

```sec
Rectangle { width }
```

means that field `width` is bound locally as `width`.

### 9.7 Borrowed field bindings

Field destructuring follows normal ownership and borrowing rules.

Shared borrow:

```sec
Rectangle {
    width: ref w
    height: ref h
} => Use(w, h)
```

Mutable borrow when the subject and borrow rules permit it:

```sec
Rectangle {
    width: ref mut w
} => Modify(w)
```

Pattern borrows are scoped according to the normal match and borrowing rules.

### 9.8 No hidden partial moves

Field destructuring must not introduce an implicit partial move that leaves a reusable union value in an invalid hidden state.

A plain by-value field binding is valid only when the field type is implicitly
copyable. It copies the field; it never silently moves a move-only field out of
the reusable union payload.

When a field is move-only, the programmer must use the appropriate `ref` or
`ref mut` binding for field access, or bind the complete variant payload by
value when ownership transfer is required.

The compiler must not silently clone or borrow payload data, and shallow field
destructuring must not create a hidden partially moved reusable union subject.

### 9.9 No nested recursive pattern language

This rulebook does not introduce recursive nested destructuring syntax such as:

```sec
Some(Ok(value)) => ...
```

or nested struct/union patterns inside a field binding.

Use another `match` for the nested value:

```sec
match option {
    Some(result) => {
        match result {
            Ok(value) => Process(value)
            Err(error) => Handle(error)
        }
    }

    None => Missing()
}
```

A future additive nested-pattern facility must not change the semantics defined here.

---

## 10. Matching `empty`

### 10.1 Empty pattern

When a union binding may be empty, `match` may contain the compiler-known `empty` pattern:

```sec
match state {
    empty => Initialize()
    Idle => Wait()
    Running => Work()
}
```

`empty` is not namespace-qualified because it is not a union variant.

### 10.2 Exhaustiveness with possible empty state

If control-flow analysis determines that the subject may be empty, an exhaustive match must cover:

- the reachable declared variants; and
- the reachable `empty` state;

unless an unguarded catch-all covers the remaining states.

Example:

```sec
let mut state: State

if condition {
    state = State.Idle
}

match state {
    empty => Initialize()
    Idle => Wait()
    Running => Work()
}
```

### 10.3 Known initialized subjects

When the subject is known to be initialized, `empty` is not part of exhaustiveness.

Function parameters are actual values and therefore initialized:

```sec
fn Process(state: State) void {
    match state {
        Idle => Wait()
        Running => Work()
    }
}
```

No `empty` arm is required.

### 10.4 Catch-all

An unguarded `_` pattern covers every remaining reachable state, including `empty` when `empty` is reachable.

Tooling may warn when `_` absorbs a reachable `empty` state and no explicit `empty` arm exists, because accidental handling of uninitialized state can hide a logic error.

---

## 11. Guards

Union patterns use the normal match guard syntax:

```sec
match message {
    Move { x, y } where x == y => HandleDiagonal(x)
    Move { x, y } => HandleMove(x, y)
    _ => Other()
}
```

A guarded arm does not by itself exhaust the underlying variant or empty state.

Bindings introduced by the pattern are visible in the guard.

---

## 12. Result and Option

`Result[T, E]` and `Option[T]` follow their own compiler-known/core-library rules while using the same union-style matching model where specified.

`None` is an actual `Option[T]` value.

It is not `empty`.

`Err(error)` is an actual `Result[T, E]` variant.

It is not `empty`.

Use `try` for propagation according to the error-handling rules.

Use `match` when the program needs to inspect and handle `Ok`/`Err` or `Some`/`None` explicitly.

Do not use the union `empty` state as a replacement for `Option.None`, `Result.Err`, or ordinary error handling.

---

## 13. Assignment and conversion

A union value may be assigned only where its exact union type is assignable.

Valid:

```sec
let value: Number := Number.Integer(42)
```

Invalid implicit payload-to-union conversion:

```sec
let value: Number := 42
```

Invalid implicit union-to-payload conversion:

```sec
let value: int := Number.Integer(42)
```

Distinct union types are not implicitly interchangeable even when their variants or payloads look identical.

Explicit casts between a union and one of its payload types are not defined by this rulebook.

Explicit casts between distinct union types are not defined by this rulebook.

---

## 14. Equality

Two values may be compared with ordinary equality only when:

1. they have the same union type; and
2. every possible payload required for union equality is equality-comparable under the normal type rules.

Payload-less unions are equality-comparable.

Variant identity participates in equality.

Example:

```sec
type Value union {
    First(int)
    Second(int)
}
```

Then:

```sec
Value.First(1) != Value.Second(1)
```

because the active variants differ.

`empty` does not participate in union value equality because it is not a union value.

---

## 15. Copy and move behavior

A union is copyable only when all variant payload possibilities satisfy the normal copy rules.

A payload-less union is copyable.

A union containing any non-copyable possible payload is not generally copyable as a value.

Assignment follows normal copy/move semantics.

Pattern binding follows the ownership and borrowing rules and must not introduce hidden copying, cloning, or invalid partial moves.

---

## 16. Generic unions

Generic unions use the normal generic parameter syntax:

```sec
type Option[T] union {
    Some(T)
    None
}
```

```sec
type Result[T, E] union {
    Ok(T)
    Err(E)
}
```

Generic parameters may appear in unnamed payloads, struct-like payload fields, nested generic references, references, arrays, slices, and other valid type forms.

Each concrete generic instantiation is a distinct concrete union type.

Constructor syntax and inference follow the generic construction rules.

---

## 17. Nested unions

A union may be nested where the general declaration and implementation rules permit nested type declarations.

Example:

```sec
type Token struct {
}

impl Token {
    type Kind union {
        Identifier
        Number
        Symbol
    }
}
```

Outside the owner implementation:

```sec
Token.Kind.Identifier
```

The nesting relationship affects naming and lookup, not the fundamental union semantics.

---

## 18. Recursive unions

Direct recursive storage is invalid because it has infinite size.

Invalid:

```sec
type List[T] union {
    Node {
        value: T
        next: List[T]
    }

    End
}
```

Recursion through a valid finite indirection is allowed:

```sec
type List[T] union {
    Node {
        value: T
        next: Box[List[T]]
    }

    End
}
```

The exact legality of an indirection form is defined by the corresponding storage, ownership, and type rules.

---

## 19. Memory model and representation

A concrete initialized union value conceptually requires:

- an active-variant discriminator; and
- storage sufficient and aligned for the active payload representation.

Each variant has a stable compiler-internal identity derived from declaration order or equivalent canonical metadata.

Those identities are implementation metadata, not source-level numeric values.

The compiler may optimize the physical representation when the optimization preserves all observable semantics.

### 19.1 Empty-state representation

The `empty` initialization state is not a declared union tag.

When runtime control flow requires empty-state observation, the compiler may represent initialization separately using an SSA flag, initialization bit, control-flow state, or an equivalent internal mechanism.

That mechanism:

- must not become a source-visible union variant;
- must not change union equality;
- must not change the declared variant set;
- must not escape through normal union value passing.

---

## 20. Implementations

An `extern "C" type Name union { ... }` is a distinct foreign representation,
not an ordinary tagged Sec union. Its fields overlap under the active C ABI, it
has no hidden tag or compiler-tracked active variant, and direct member access
is classified as foreign-unsafe. Ordinary Sec unions remain closed, tagged, and
governed by the rules above.

---

A union is a nominal Sec type and may participate in ordinary `impl` declarations where permitted by the implementation rulebook.

This rulebook does not duplicate general method, property, or interface implementation semantics.

Variants are constructors and pattern alternatives, not independent nominal types with their own separate implementation blocks.

---

## 21. Visibility

Union type visibility follows the normal named-type visibility rules.

Variant visibility follows the variant declaration and owner union visibility rules.

A public union must not expose payload types that violate the normal visibility rules for public APIs.

---

## 22. Parser requirements

The parser must support:

- `type Name union`;
- optional generic parameters;
- payload-less variants;
- one unnamed payload type;
- struct-like named payload fields;
- optional variant separators according to ordinary Sec declaration grammar;
- `default` after a variant declaration;
- whole-payload union patterns;
- shallow named-field union patterns;
- field-binding rename;
- `ref` and `ref mut` field bindings where pattern borrowing is legal;
- the compiler-known `empty` union pattern;
- `is Variant`;
- `is empty`.

The AST must preserve:

- union source location;
- variant declaration order;
- variant payload kind;
- payload types/fields;
- the optional default marker;
- pattern field bindings and binding modes.

---

## 23. Name-resolution requirements

The compiler must:

- register the union type name;
- register generic arity when applicable;
- register the nested owner relationship when applicable;
- register variant names in the union namespace;
- reject duplicate variants;
- resolve every payload type;
- reject unknown payload types;
- reject duplicate struct-like payload field names;
- validate visibility;
- detect invalid recursive storage;
- resolve contextual unqualified union variant names in `match` and `is` expressions;
- reject ambiguous variant resolution.

---

## 24. Sema requirements

Sema must:

- treat unions as nominal closed types;
- reject empty union declarations;
- validate every variant payload;
- validate explicit default markers;
- reject more than one default variant;
- compute union defaultability from the marked default variant;
- default-initialize an explicitly defaultable mutable union declaration;
- allow the union-specific empty-state exception for mutable NonDefaultable union declarations without an initializer;
- track empty / initialized / moved definite-assignment state;
- prevent empty state from escaping as a union value;
- validate `is Variant` and `is empty` tests;
- refine control-flow state after successful and failed `is` tests where provable;
- validate variant construction;
- apply default values to omitted struct-like payload fields when legal;
- reject omitted NonDefaultable payload fields;
- reject unknown and duplicate construction fields;
- integrate all variant forms with `match`;
- support whole-payload and shallow named-field destructuring;
- validate partial field bindings and rename syntax;
- validate field-level `ref` / `ref mut` borrowing;
- prevent hidden partial moves;
- include reachable `empty` state in match exhaustiveness;
- exclude impossible `empty` state from exhaustiveness when initialization is proven;
- treat `_` as covering any remaining reachable state;
- calculate copyability and equality-comparability;
- preserve concrete generic substitutions;
- provide backend layout metadata;
- keep internal variant tags unavailable as source-level integers.

---

## 25. Code-generation requirements

For every concrete initialized union type, lowering must know enough to represent:

- the concrete variant set;
- payload layout requirements;
- discriminator information;
- size and alignment;
- copy/move/destruction requirements.

Variant construction must initialize exactly the selected variant and payload.

Match lowering must branch according to the active variant and, when relevant, the compiler-known empty initialization state.

Payload bindings must preserve ownership and borrow semantics.

The backend must not read uninitialized payload storage.

The backend must not expose `empty` as a hidden source-level variant.

---

## 26. Required diagnostics

Diagnostics must cover at least:

- anonymous union declarations;
- empty union declarations;
- duplicate variant names;
- unknown payload types;
- duplicate struct-like payload fields;
- invalid direct recursion;
- invalid payload construction arity;
- invalid payload value types;
- missing NonDefaultable struct-like payload fields;
- unknown construction fields;
- duplicate construction fields;
- multiple default variants;
- non-default-constructible selected default variant;
- read/use/borrow/pass/return of an empty union binding where an actual value is required;
- invalid `empty` use on a type or expression that cannot have empty initialization state;
- impossible state tests according to the normal analysis policy;
- unknown variant in `is` or `match`;
- duplicate or unreachable match coverage;
- non-exhaustive union matches, including reachable `empty` state;
- invalid field destructuring names;
- duplicate destructured fields;
- illegal borrowed field bindings;
- hidden/illegal partial moves;
- visibility violations.

Diagnostic IDs are assigned according to the central diagnostics rulebook.

---

## 27. Required tests

The test suite must cover at least:

1. payload-less unions;
2. single-payload unions;
3. struct-like payload unions;
4. mixed variant kinds;
5. generic unions;
6. nested unions;
7. duplicate variants;
8. duplicate payload fields;
9. direct recursion rejection;
10. recursion through legal indirection;
11. explicit default payload-less variant;
12. explicit default single payload;
13. explicit default struct-like payload;
14. rejection of non-default-constructible default variant;
15. rejection of multiple defaults;
16. NonDefaultable mutable union empty declaration;
17. `is empty`;
18. `is Variant` for payload-less and payload variants;
19. data-flow refinement after `is empty`;
20. empty-state match coverage;
21. `_` covering reachable empty state;
22. rejection of empty-state escape;
23. moved-from not being treated as empty;
24. omitted Defaultable struct-like construction fields;
25. rejection of omitted NonDefaultable fields;
26. whole-payload match binding;
27. shallow named-field destructuring;
28. partial named-field destructuring;
29. destructuring rename;
30. `ref` destructuring;
31. `ref mut` destructuring;
32. hidden partial-move rejection;
33. nested destructuring remaining non-canonical;
34. match exhaustiveness across all variants;
35. equality derivation;
36. copy/move classification;
37. destruction of active payloads;
38. no-op destruction for still-empty local union storage.

---

## 28. Design principles

The union model follows these principles:

1. **Closed value domain.** Only declared variants are union values.
2. **Explicit payload construction.** The selected variant is always visible in construction.
3. **Explicit payload inspection.** `match` performs exhaustive inspection and extraction.
4. **Cheap state test.** `is Variant` checks one variant without requiring a full match.
5. **No fake nullability.** `empty` is initialization state, not a null union value.
6. **No hidden variant.** Empty storage never changes the declared union domain.
7. **Default only by intent.** A union has a semantic default only when one variant is explicitly marked `default`.
8. **Defaultable payload ergonomics.** Struct-like payload construction may omit fields whose types already have valid defaults.
9. **Shallow destructuring.** Named variant fields may be bound directly without introducing a recursive pattern language.
10. **No hidden ownership effects.** Matching and destructuring must never silently clone or create invalid partial moves.
