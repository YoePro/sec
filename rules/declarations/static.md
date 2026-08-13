# Static

## Purpose

`static` declares storage or members that belong to a type, function or program
rather than to one ordinary instance.

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

---

## Core meanings

`static` has two related uses:

1. static storage duration
2. type-associated membership

Static storage exists independently of an ordinary local scope or object
instance.

A static member belongs to a type rather than to one instance of that type.

---

## No const keyword

Sec does not use a `const` keyword.

Immutable declarations use:

```sec
let
```

Mutable declarations use:

```sec
let mut
```

Static immutable storage therefore uses:

```sec
static let
```

Static mutable storage uses:

```sec
static let mut
```

---

## Module-level declarations

Module-level bindings have static storage duration implicitly.

Example:

```sec
let Configuration: ApplicationConfiguration
let mut State: ApplicationState
```

These values exist for the lifetime defined for the module or program.

The programmer may write `static` explicitly:

```sec
static let Configuration: ApplicationConfiguration
static let mut State: ApplicationState
```

The explicit modifier is semantically redundant at module level.

The compiler should accept it.

The formatter should remove it.

Formatted result:

```sec
let Configuration: ApplicationConfiguration
let mut State: ApplicationState
```

The compiler may emit an informational diagnostic:

```text
static is redundant on module-level declaration State
```

This should not normally be a warning or error.

---

## Function-local static storage

A local declaration may use `static` to preserve one storage location across
function calls.

Example:

```sec
fn NextID() int {
    static let mut value: int := 0

    value += 1
    return value
}
```

`value`:

- is visible only inside `NextID`
- is initialized once
- retains its value between calls
- is shared by all concurrent calls to `NextID`
- has static storage duration

It is not allocated separately for each invocation.

---

## Static local instances

Any valid value type may use function-local static storage.

Example:

```sec
fn State() ref ApplicationState {
    static let mut state: ApplicationState := ApplicationState {
        running: false
    }

    return ref state
}
```

Returning a reference to static storage may be valid because the storage outlives
the function call.

Lifetime validity does not imply concurrency safety.

Mutable static storage still requires valid synchronization.

Static mutable storage accessed by tasks, threads or mixed task/thread execution
requires valid synchronization.

Examples include:

- `Mutex[T]`
- `Atomic[T]`
- channel ownership transfer
- another compiler-approved synchronization primitive

Using a physical thread does not bypass static mutability or borrowing rules.

Sec v0.1 does not define thread-local storage syntax.

---

## Static declarations in impl

Static type-associated storage belongs in `impl`.

Example:

```sec
type Counter struct {
    value: int
}

impl Counter {
    static let Maximum: int := 100
    static let mut Total: int := 0
}
```

Static declarations do not belong in the struct field list.

This preserves the distinction:

```text
type
    instance data and physical layout

impl
    behavior and type-associated members
```

---

## Static storage and instance layout

Static members are not instance fields.

Example:

```sec
type Counter struct {
    value: int
}

impl Counter {
    static let mut Total: int := 0
}
```

`Counter.Total` does not affect:

- `sizeof(Counter)`
- alignment of `Counter`
- field offsets
- ABI layout
- object construction

Every `Counter` instance contains only its declared instance fields.

---

## Static access

Static members are accessed through the type name.

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

The compiler should require type-qualified access to make shared type-level state
explicit.

Suggested diagnostic:

```text
static member Total must be accessed through type Counter
```

---

## Static methods

A method that belongs to the type rather than an instance must be declared
explicitly with:

```sec
static fn
```

Example:

```sec
impl Counter {
    static fn Create() Counter {
        return Counter {
            value: 0
        }
    }
}
```

Sec does not infer static membership merely because `self` is absent.

This is invalid:

```sec
impl Counter {
    fn Create() Counter {
        return Counter {
            value: 0
        }
    }
}
```

Explicit `static fn` is required for semantic clarity.

---

## Instance methods

An ordinary method belongs to an instance and receives compiler-provided
implicit `self`; it must not declare a receiver parameter.

Example:

```sec
impl Counter {
    fn Increment() void {
        self.value += 1
    }
}
```

A static method must not use `self`.

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

---

## Static properties

A property that belongs to a type must be declared explicitly:

```sec
static property
```

Example:

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

Access:

```sec
let count := Counter.Count
```

A static property has no instance receiver.

It must not use `self`.

Expected diagnostic:

```text
self cannot be used in static property Count
```

---

## Instance properties

An ordinary property belongs to an instance.

Example:

```sec
impl Counter {
    property Value: int {
        get {
            return self.value
        }
    }
}
```

Access:

```sec
let value := counter.Value
```

An instance property may use `self`.

A property without `static` must resolve against an instance.

---

## Static setters

A static property may define a setter.

Example:

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

The setter operates on type-associated storage.
Its incoming value parameter is always explicit and programmer-named. The same
rule applies to `try set name`; there is no implicit `value` binding.

It must obey:

- mutability rules
- validation rules
- concurrency rules
- property rules
- visibility rules

---

## Static generic members

Generic types may declare static members.

Example:

```sec
impl Cache[T] {
    static fn Create() Cache[T] {
        return Cache[T] {}
    }
}
```

Whether static storage is shared across all type arguments or specialized per
concrete generic instantiation must be defined explicitly.

Version 0.1 should use:

```text
Static storage in a generic impl is specialized per concrete generic type.
```

Therefore:

```sec
Cache[int].Count
Cache[string].Count
```

refer to distinct static storage locations.

---

## Static named types

Named types may define static methods, properties and storage.

Example:

```sec
type UserID int

impl UserID {
    static let Invalid: UserID := UserID(-1)

    static fn Parse(value: string) Result[UserID, ParseError] {
    }
}
```

The same static rules apply to:

- structs
- enums
- unions
- registers
- named types
- generic types
- other valid impl targets

---

## Static initialization

Static initialization must be deterministic and visible to semantic analysis.

Version 0.1 should allow static initializers that are compile-time evaluable.

Example:

```sec
static let mut Count: int := 0
```

Runtime initialization must not occur through hidden startup functions.

Invalid as an implicit static initializer:

```sec
static let Configuration := LoadConfiguration()
```

when `LoadConfiguration()` requires runtime execution.

Runtime initialization must be explicit in ordinary program flow.

Example:

```sec
let mut Configuration: Option[ApplicationConfiguration] := None

fn main() Result[void, StartupError] {
    Configuration = Some(try LoadConfiguration())
    return Ok()
}
```

Sec must not introduce hidden module or static startup initializers. The
explicit `init(...)` lifecycle member defined by `impl.md` constructs an
instance and does not authorize hidden static/module initialization.

---

## Initialization order

Static initialization order must not depend on source-file discovery order.

Compile-time static initialization may be resolved from dependency analysis.

Cycles are invalid.

Example:

```sec
static let A: int := B
static let B: int := A
```

Expected diagnostic:

```text
cyclic static initialization: A -> B -> A
```

Runtime initialization remains explicit and follows ordinary control flow.

---

## Static mutability

`static let` is immutable after initialization.

Example:

```sec
static let Maximum: int := 100
```

`static let mut` is mutable.

Example:

```sec
static let mut Total: int := 0
```

Mutability does not imply thread safety.

Static mutable storage is shared mutable state whenever multiple tasks may access
it.

---

## Concurrency

Static lifetime solves lifetime requirements.

It does not solve synchronization.

Invalid concurrent access:

```sec
static let mut Total: int := 0

fn Increment() void {
    Total += 1
}
```

when multiple tasks may execute `Increment()` concurrently.

Preferred synchronized form:

```sec
static let Total: Mutex[int] := Mutex(0)
```

or an appropriate atomic type.

The compiler should diagnose statically provable unsynchronized shared mutation.

---

## Static Mutex storage

A static mutex is the preferred form for shared structured mutable state.

```sec
static let State: Mutex[ApplicationState] := Mutex(
    ApplicationState {
        running: false
    }
)
```

The mutex binding itself is immutable.

Mutation occurs through:

```sec
let mut state := State.lock()
state.running = true
```

This normally avoids:

```sec
static let mut State: Mutex[ApplicationState]
```

because replacing the shared mutex is usually invalid or unnecessary.

---

## Detached tasks

Detached tasks may use references to static storage when:

- the storage outlives the task
- access is synchronized correctly
- shutdown ordering is valid
- the value is not destroyed while still in use

Example:

```sec
static let State: Mutex[ApplicationState]

fn Worker() void {
    let mut state := State.lock()
    state.running = true
}
```

Static lifetime does not permit unsynchronized mutable references.

---

## References to static storage

A reference to immutable static storage may be returned or stored when the type
and access are concurrency-safe.

Example:

```sec
static let Configuration: ApplicationConfiguration

fn GetConfiguration() ref ApplicationConfiguration {
    return ref Configuration
}
```

A mutable reference to shared static storage must not bypass synchronization.

Invalid:

```sec
static let mut State: ApplicationState

fn GetState() ref mut ApplicationState {
    return ref mut State
}
```

unless the compiler can prove exclusive program-wide access.

---

## Static destruction

Static values normally live until program shutdown.

Destruction order must be based on dependency and use relationships rather than
arbitrary source order.

A static value must not be destroyed while:

- a detached task may use it
- a reference remains active
- a mutex guard remains active
- another static destructor depends on it

Normal shutdown may perform deterministic destruction.

Forced termination does not guarantee static destruction.

Examples include:

- Unix `SIGKILL`
- power loss
- hardware reset
- kernel failure

---

## Target profiles

Hosted targets may place static storage in normal process data sections.

Embedded and bare-metal targets may place static storage in:

- `.data`
- `.bss`
- ROM
- flash
- target-specific memory sections

The target profile may restrict:

- static memory size
- writable static storage
- initialization mechanisms
- destruction support
- thread-safe access

These restrictions must not change source-level ownership semantics.

---

## Static and memory sections

A future attribute system may allow explicit placement.

Conceptual examples:

```sec
@section(".fast")
static let mut Buffer: byte[1024]
```

```sec
@section(".persistent")
static let mut State: PersistentState
```

This is not required for version 0.1.

`static` itself declares lifetime and association, not physical section
placement.

---

## Shadowing

A local declaration may shadow a static declaration only when normal visibility
rules permit it.

Example:

```sec
let State: int := 1

fn Example() void {
    let State: int := 2
}
```

The compiler or linter may diagnose confusing shadowing.

Type-qualified static members remain unambiguous:

```sec
Application.State
```

---

## Visibility

Static members follow ordinary Sec visibility rules.

This includes rules for:

- public names
- identifiers beginning with `_`
- identifiers beginning with `__`
- module visibility
- impl visibility

`static` does not change visibility.

---

## Formatter behavior

The formatter should:

- preserve required `static`
- remove redundant module-level `static`
- normalize canonical keyword spacing
- preserve `static fn`
- preserve `static property`
- preserve `static let`
- preserve `static let mut`

Example input:

```sec
static let mut State: ApplicationState
```

at module level may format to:

```sec
let mut State: ApplicationState
```

Inside a function or `impl`, `static` must remain.

---

## Semantic analysis

The compiler must determine:

- whether static is valid in the declaration context
- whether static is redundant
- whether a member is static or instance-bound
- whether `self` use is valid
- whether initialization is compile-time valid
- whether initialization has cycles
- whether mutable static access is synchronized
- whether references outlive static destruction
- whether generic static storage is specialized correctly
- whether static storage affects instance layout

---

## Semantic IR

Semantic IR must represent static declarations explicitly.

At minimum:

```text
StaticStorage
StaticLoad
StaticStore
StaticMethod
StaticProperty
StaticInitialize
StaticDestroy
```

IR must record:

- owner module or type
- concrete value type
- mutability
- visibility
- initialization dependency
- generic specialization
- concurrency requirements
- source location

Static members must not be represented as instance fields.

---

## Diagnostics

Examples:

```text
static is redundant on module-level declaration State
```

```text
self cannot be used in static method Create
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

---

## Restrictions

`static` must not:

- be used as a type modifier
- silently make access thread-safe
- add hidden runtime initialization
- add hidden lazy initialization
- add hidden locking
- affect struct instance layout
- permit `self` in static methods
- permit `self` in static properties
- bypass ownership or borrowing rules
- bypass shutdown ordering
- imply physical memory-section placement

---

## Related rules

Detailed behavior is defined in:

```text
impl.md
properties.md
tasks.txt
concurrency.txt
mutex.txt
atomics.txt
concurrency_memory_model.txt
```

## Current implementation status

Implemented:

- `static` is a lexer keyword.
- parser accepts `static let`.
- parser accepts `static fn`.
- parser accepts `static let` and `static fn` inside `impl`.
- sema marks `static let` storage as `Static`.
- function-local `static let` is accepted as static storage.
- `impl Type { static let Name ... }` is registered as type-associated static
  storage and can be read as `Type.Name`.

Not implemented yet:

- formatter removal of redundant module-level `static`
- static initializer purity/compile-time validation
- cyclic static initialization checks
- mutable static synchronization diagnostics
- full static method distinction in overload/member lookup
- static member assignment through `Type.Name`
- MLIR/LLVM lowering for static storage
