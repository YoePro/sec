# Sec Static Declarations and Members

- **Status:** Normative
- **Created:** 2026-08-13
- **Last updated:** 2026-08-14
- **Document revision:** 2.0
- **Language version:** Sec 0.1
- **Supersedes:** document revision 1.0
- **Canonical path:** `rules/declarations/static.md`

## 1. Purpose

`static` declares storage or members that belong to a program, function, or type rather than to one ordinary instance.

`static` is a declaration modifier.

It is not a type modifier.

Valid:

```sec
static let State: ApplicationState
```

Invalid:

```sec
let State: static ApplicationState
```

`static` has two related meanings:

1. static storage duration;
2. type-associated membership.

These meanings are related but distinct.

## 2. Immutable and mutable static storage

Sec has no `const` keyword.

Immutable static storage uses:

```sec
static let Maximum: int := 100
```

Mutable static storage uses:

```sec
static let mut Total: int := 0
```

`static let` is immutable after initialization.

`static let mut` permits mutation subject to the ordinary ownership, borrowing, effect, and concurrency rules.

Mutability never implies thread safety.

## 3. Module-level storage

Module-level bindings already have static storage duration.

```sec
let Configuration: ApplicationConfiguration
let mut State: ApplicationState
```

Writing `static` explicitly at module scope is accepted but semantically redundant.

```sec
static let Configuration: ApplicationConfiguration
static let mut State: ApplicationState
```

The formatter should remove redundant module-level `static`.

Formatted form:

```sec
let Configuration: ApplicationConfiguration
let mut State: ApplicationState
```

A compiler or LSP may emit an informational diagnostic.

```text
static is redundant on module-level declaration State
```

This is not normally a warning or error.

## 4. Function-local static storage

A function-local declaration may use `static` when one storage location must persist across calls.

```sec
fn NextID() int {
    static let mut value: int := 0

    value += 1
    return value
}
```

The static local:

- is visible only in the declaring lexical scope;
- is initialized once from a compile-time-valid initializer;
- retains its value across function invocations;
- is shared by all executions that reach the same declaration;
- has static storage duration;
- is not allocated separately for each invocation.

Function-local static storage is not thread-local storage.

Sec 0.1 defines no thread-local static syntax.

## 5. References to static storage

Static lifetime may make a reference lifetime-valid.

```sec
fn State() ref ApplicationState {
    static let state: ApplicationState := ApplicationState {
        running: false
    }

    return ref state
}
```

Lifetime validity does not imply synchronization safety.

A mutable reference to shared mutable static storage must not bypass required synchronization.

Invalid unless exclusive access can be proven:

```sec
static let mut State: ApplicationState

fn GetState() ref mut ApplicationState {
    return ref mut State
}
```

## 6. Static declarations in implementations

Type-associated static members belong in `impl`.

```sec
type Counter struct {
    value: int
}

impl Counter {
    static let Maximum: int := 100
    static let mut Total: int := 0
}
```

A static declaration may also appear in an `impl extends` fragment.

```sec
impl extends Counter {
    static fn ResetTotal() void {
        Counter.Total = 0
    }
}
```

Primary and extended implementation fragments form one combined member surface.

Duplicate or conflicting static members are invalid across the complete implementation.

## 7. Static members never change instance representation

Static members are not instance fields.

```sec
type Counter struct {
    value: int
}

impl Counter {
    static let mut Total: int := 0
}
```

`Counter.Total` does not change:

- `Counter.SizeOf`;
- alignment of `Counter`;
- instance field offsets;
- ABI layout of `Counter`;
- construction or destruction of one ordinary `Counter` instance.

Stored instance representation is defined by the type declaration.

Static state is separate storage.

## 8. Static member access

Static members are accessed through the type.

Valid:

```sec
Counter.Total
Counter.Create()
Counter.Count
```

Invalid:

```sec
counter.Total
counter.Create()
counter.Count
```

The compiler must require type-qualified access to make shared type-level state explicit.

Suggested diagnostic:

```text
static member Total must be accessed through type Counter
```

## 9. Static methods

A type-level method is declared explicitly with `static fn`.

```sec
impl Counter {
    static fn Create() Counter {
        return Counter {
            value: 0
        }
    }
}
```

`static fn` has no instance receiver and may not use `self`.

Invalid:

```sec
impl Counter {
    static fn Reset() void {
        self.value = 0
    }
}
```

Expected diagnostic:

```text
self cannot be used in static method Reset
```

### 9.1 Ordinary instance methods

An ordinary method is declared with `fn`.

```sec
impl Counter {
    fn Increment() void {
        self.value += 1
    }
}
```

Instance methods have implicit `self`.

The programmer does not declare `self`, `ref self`, or `ref mut self`.

Sema infers receiver requirements from the method body.

The absence of explicit `self` in the parameter list does not make a method static.

Only `static fn` declares a static method.

## 10. Static functions and lifecycle construction

A static function may act as a named factory or helper.

```sec
impl Counter {
    static fn Zero() Counter {
        return Counter {
            value: 0
        }
    }
}
```

A static function is not a lifecycle `init` member.

Lifecycle construction through `init` is selected explicitly with `new`.

```sec
let counter := new Counter(...)
```

`new` does not imply heap allocation.

The lifecycle rules are defined by `impl.md`.

## 11. Static properties

A type-level property is declared with `static property`.

```sec
impl Counter {
    static let mut Total: int := 0

    static property Count: int {
        get {
            return Counter.Total
        }
    }
}
```

A static property:

- has no instance receiver;
- may not use `self`;
- is accessed through the type;
- follows the ordinary property accessor rules.

## 12. Static property setters

Static property setters always declare their incoming value parameter explicitly.

```sec
impl Application {
    static let mut _mode: Mode := Mode.Normal

    static property Mode: Mode {
        get {
            return Application._mode
        }

        set mode {
            Application._mode = mode
        }
    }
}
```

Fallible form:

```sec
static property Mode: Mode {
    try set mode {
        ...
    }
}
```

The parameter name is programmer-chosen.

There is no implicit setter variable.

Invalid:

```sec
static property Mode: Mode {
    set {
        Application._mode = value
    }
}
```

The detailed property rules are defined by `properties.md`.

## 13. Static members on nominal types

Any eligible nominal `impl` target may own static members.

This includes, where permitted by the corresponding type rules:

- structs;
- enums;
- unions;
- registers;
- named scalar types;
- generic nominal types;
- other valid implementation targets.

Example:

```sec
type UserID int

impl UserID {
    static let Invalid: UserID := UserID(-1)

    static fn Parse(value: string) Result[UserID, ParseError] {
        ...
    }
}
```

Ordinary `impl` ownership remains restricted to the type's defining module.

## 14. Generic static members

Generic implementations may declare static members.

```sec
impl Cache[T] {
    static let mut Count: int := 0
}
```

Static storage in a generic implementation is specialized per concrete generic instantiation.

Therefore:

```sec
Cache[int].Count
Cache[string].Count
```

refer to distinct static storage locations.

The same rule applies to function-local static storage inside monomorphized generic functions unless a more specific generic rule says otherwise.

No implicit storage is shared across all generic instantiations merely because the source declaration is shared.

## 15. Static initialization

Static initialization must be deterministic and visible to semantic analysis.

A static initializer must be compile-time evaluable under the compile-time evaluation rules.

Valid:

```sec
static let mut Count: int := 0
```

A runtime operation must not be hidden inside static initialization.

Invalid when `LoadConfiguration()` requires runtime execution:

```sec
static let Configuration := LoadConfiguration()
```

Runtime setup must occur explicitly in ordinary program flow.

```sec
let mut Configuration: Option[ApplicationConfiguration] := None

fn main() Result[void, StartupError] {
    Configuration = Some(try LoadConfiguration())
    return Ok()
}
```

Normative rule:

> Static declarations must not cause hidden runtime startup initialization. Runtime initialization occurs only through explicit program flow.

This rule is unrelated to lifecycle `init` members. `init` constructs an explicitly requested instance; it is not an implicit static-startup mechanism.

## 16. Static initialization dependency order

Static initialization order must not depend on source-file discovery order.

Compile-time static initializers may depend on other compile-time static values.

The compiler must resolve initialization dependencies.

Dependency cycles are invalid.

```sec
static let A: int := B
static let B: int := A
```

Expected diagnostic:

```text
cyclic static initialization: A -> B -> A
```

Because runtime static initialization is forbidden, there is no hidden runtime initialization-order model.

## 17. Static mutability and concurrency

Static lifetime solves lifetime duration.

It does not solve synchronization.

Unsynchronized shared mutation is invalid when concurrent access is possible.

```sec
static let mut Total: int := 0

fn Increment() void {
    Total += 1
}
```

If multiple tasks or threads may execute `Increment()` concurrently, the access requires a synchronization strategy.

Examples include:

- `Mutex[T]`;
- `Atomic[T]`;
- ownership transfer through a channel;
- another compiler-approved synchronization primitive.

The compiler should diagnose statically provable unsynchronized shared mutation.

## 18. Static synchronized storage

An immutable binding to a synchronization primitive is normally preferred over replacing the primitive itself.

```sec
static let State: Mutex[ApplicationState] := Mutex(
    ApplicationState {
        running: false
    }
)
```

Mutation then occurs through the primitive's access mechanism.

```sec
let mut state := State.lock()
state.running = true
```

Static storage does not weaken the concurrency memory model.

## 19. Detached execution and static storage

Detached tasks or threads may use static storage only when:

- the storage outlives the execution;
- access obeys synchronization requirements;
- shutdown ordering remains valid;
- destruction cannot occur while the storage is still in use.

Static lifetime alone never authorizes unsynchronized mutable references.

## 20. Static destruction

Static values normally live until program shutdown.

Deterministic destruction, when supported by the selected target profile, must follow dependency and use relationships rather than arbitrary source order.

A static value must not be destroyed while:

- a live reference still depends on it;
- a detached task or thread may still access it;
- a synchronization guard remains active;
- another static destruction path still depends on it.

Forced termination does not guarantee static destruction.

Examples include:

- process termination that bypasses cleanup;
- power loss;
- hardware reset;
- kernel or runtime failure.

Static destruction follows the general destruction rules.

## 21. Target profiles

Hosted targets may place static storage in ordinary process data sections.

Embedded and bare-metal targets may place static storage in target-defined regions such as:

- initialized writable data;
- zero-initialized storage;
- ROM;
- flash;
- explicitly selected target storage.

A target profile may restrict:

- total static storage size;
- writable static storage;
- supported placement;
- destruction support;
- concurrency behavior.

These restrictions must not change source-level ownership semantics.

## 22. Physical placement is separate from `static`

`static` defines lifetime and association.

It does not by itself define:

- a linker section;
- an absolute address;
- a memory space;
- an MMIO binding;
- a target-specific placement class.

Physical placement is governed by the canonical attribute, storage, ABI, or platform rules when such a mechanism is defined.

Sec 0.1 uses a closed set of compiler-known attributes. A placement syntax must not be assumed merely because a conceptual attribute name would be convenient.

## 23. Shadowing

Ordinary visibility and shadowing rules apply.

A local declaration may shadow a module static when normal scope rules permit it.

```sec
let State: int := 1

fn Example() void {
    let State: int := 2
}
```

The compiler or analysis layer may diagnose confusing shadowing.

Type-qualified static members remain unambiguous.

```sec
Application.State
```

## 24. Visibility

`static` does not change visibility.

Static members follow the ordinary visibility rules for:

- public names;
- module visibility;
- implementation visibility;
- underscore-prefixed names;
- other visibility categories defined by the language.

## 25. Formatter behavior

The formatter must:

- preserve required `static`;
- remove redundant module-level `static`;
- preserve `static fn`;
- preserve `static property`;
- preserve `static let`;
- preserve `static let mut`;
- preserve explicit static-property setter parameters;
- never rewrite an instance `fn` into `static fn` merely because `self` is not textually referenced.

## 26. Semantic analysis

The compiler must determine at least:

- whether `static` is valid in the declaration context;
- whether module-level `static` is redundant;
- whether a member is static or instance-bound;
- whether `self` use is invalid in a static member;
- whether a static initializer is compile-time evaluable;
- whether initialization dependencies contain a cycle;
- whether mutable static access is concurrency-safe;
- whether references remain valid through static destruction;
- whether generic static storage is specialized per concrete instantiation;
- whether a static member is accessed through the type;
- whether static declarations affect instance layout;
- whether duplicate members exist across primary and extended implementation fragments.

## 27. Semantic IR

Semantic IR must preserve static semantics explicitly.

At minimum it must represent or make unambiguously derivable:

```text
StaticStorage
StaticLoad
StaticStore
StaticMethod
StaticProperty
StaticInitialize
StaticDestroy
```

Static IR metadata must include, where applicable:

- owner module or type;
- concrete value type;
- mutability;
- visibility;
- initialization dependency;
- generic specialization;
- concurrency requirements;
- source location.

Static members must never be lowered as instance fields.

`StaticInitialize` represents compile-time-resolved static initialization semantics. It must not imply a hidden runtime startup call.

## 28. Diagnostics

Required diagnostics include at least:

```text
static is redundant on module-level declaration State
```

```text
self cannot be used in static method Reset
```

```text
self cannot be used in static property Count
```

```text
static member Total must be accessed through type Counter
```

```text
runtime call LoadConfiguration is not permitted in static initializer
```

```text
cyclic static initialization: A -> B -> A
```

```text
shared mutable access to static State requires synchronization
```

```text
cannot return mutable reference to shared static State
```

Diagnostics must identify the relevant declaration and violated rule.

## 29. Restrictions

`static` must not:

- be used as a type modifier;
- silently make access thread-safe;
- add hidden runtime startup initialization;
- add hidden lazy initialization;
- add hidden locking;
- change instance layout;
- permit `self` inside a static member;
- bypass ownership or borrowing;
- bypass shutdown ordering;
- imply physical address or linker-section placement.

## 30. Best practice

- Use module-level bindings directly instead of redundant `static`.
- Use function-local `static` only when one persistent storage location is genuinely part of the function's semantics.
- Prefer immutable static bindings whenever possible.
- Prefer synchronization objects over exposing mutable references to shared static state.
- Keep type-associated static API close to the type's primary behavior; use `impl extends` when splitting a large implementation improves readability.
- Use `static fn` only for genuinely type-level behavior.
- Do not use a static factory where lifecycle `new Type(...)` better expresses actual construction through `init`.
- Keep runtime setup explicit in ordinary program flow.
- Do not invent placement conventions in source; use only placement mechanisms defined by the canonical platform/attribute rules.

## 31. Cross-rulebook ownership

This rulebook owns static storage duration and static type-associated member semantics.

Related semantics are owned elsewhere:

- implementation structure, `impl extends`, `init`, `free`, and `new`: `rules/declarations/impl.md`;
- properties and explicit setter parameters: `rules/declarations/properties.md`;
- interfaces and `static fn` requirements in contracts: `rules/declarations/interfaces.md`;
- generics and monomorphization: generics rulebook;
- ownership and borrowing: memory rulebooks;
- concurrency: concurrency rulebooks;
- destruction: destruction rulebook;
- attributes and physical placement annotations: `rules/foundations/attributes.md`;
- ABI-visible layout and storage representation: platform ABI rulebook;
- Semantic IR and MLIR lowering: compiler and MLIR rulebooks.
