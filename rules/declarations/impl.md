# Impl

**Status:** Normative  
**Document revision:** 2.1
**Sec language version:** 0.1  
**Created:** 2026-08-13  
**Last updated:** 2026-09-01

**Supersedes:** `rules/declarations/impl.txt`

## Replacement notice

This document replaces `rules/declarations/impl.txt` in full.

Revision 2.0 consolidates the current implementation-fragment model, canonical
implicit `self`, associated declarations, lifecycle members, and explicit
construction through the new `new` keyword.

The introduction of `new` is a language-level syntax change. It affects lexical
reservation, parsing, AST, semantic analysis, formatting, LSP classification,
and diagnostics. Existing user identifiers named `new` become invalid once this
revision is implemented.

---

## 1. Purpose

`impl` defines behavior and type-associated declarations for a named type.

The hard separation is:

```text
type declaration
    owns stored instance representation

impl
    owns behavior and type-associated declarations
```

An `impl` must never add stored instance data to its target type or otherwise
change the target type's representation.

This rule does **not** mean that every declaration inside an `impl` must be
behavioral. An `impl` may own associated or nested type definitions because a
nested type is a separate type; defining it does not add stored data to the
implemented type.

Example:

```sec
type Vehicle struct {
    engine: Vehicle.Engine,
}

impl Vehicle {
    type Engine struct {
        MaxPower: int,
    }

    fn Start() void {
        // Behavior of Vehicle.
    }
}
```

`Vehicle.Engine.MaxPower` is stored in instances of `Vehicle.Engine`, not in
`Vehicle` merely because `Engine` is declared inside `impl Vehicle`.

Invalid:

```sec
impl Vehicle {
    engine: Engine
}
```

The `impl` above attempts to add a stored instance field to `Vehicle` and is a
semantic error.

---

## 2. Eligible targets

An ordinary `impl` targets a user-defined named nominal type.

Examples include:

- named scalar and constrained types;
- structs;
- enums;
- unions;
- registers;
- units where the unit rules expose an implementation target;
- generic named types;
- other named nominal types explicitly permitted by their rulebooks.

Example:

```sec
type Percent int range 0..100

impl Percent {
    fn IsHigh() bool {
        return self > Percent(80)
    }
}
```

An interface is not an ordinary `impl` target. Interface conformance and any
interface-specific implementation form are defined by `interfaces.md`.

Compiler/core code may define privileged implementations for compiler-known or
fundamental types where the core-library rules explicitly permit it. User code
must not use that privilege to extend compiler-known types.

---

## 3. Defining-module ownership

An ordinary implementation belongs to the module that defines the target type.

A module must not add an ordinary implementation to a named type defined by
another module.

Invalid conceptually:

```sec
module application

import graphics

impl graphics.Image {
    fn DebugDump() void {
        // Invalid: Image is owned by another module.
    }
}
```

This restriction is a coherence and protection boundary. It prevents imported
modules from changing the ordinary member surface of types they do not own.

The core/compiler privilege for compiler-known types is the only ordinary
exception and is governed by core-library rules.

---

## 4. Primary implementation

A named type has exactly one primary ordinary implementation in its defining
module:

```sec
impl Device {
    fn Start() void {
        // ...
    }
}
```

The primary implementation is the human-facing entry point for the type's
behavior.

The requirement exists primarily for readability and source organization. The
compiler may internally merge all implementation fragments into one semantic
member surface.

A second primary implementation for the same type is invalid.

---

## 5. Implementation fragments with `extends`

Additional implementation fragments use the canonical syntax:

```sec
impl extends Device {
    fn Stop() void {
        // ...
    }
}
```

The spelling is `impl extends Type`, not `impl Type extends`.

### 5.1 Same-module rule

An `impl extends Type` fragment:

- must be in the same module as the primary `impl Type`;
- may be in any source file belonging to that module;
- may appear before or after the primary implementation in source/file order;
- is invalid if no primary implementation exists in the module.

Source-file discovery order has no semantic effect.

### 5.2 Same capabilities

An extension fragment follows the same member rules as the primary
implementation.

`extends` is organizational. It does not create a weaker or stronger form of
implementation.

### 5.3 One combined member surface

The primary implementation and all valid extension fragments form one combined
type-member namespace.

Duplicates and conflicts are checked across the complete merged surface, not
per file and not per block.

Invalid:

```sec
impl Device {
    fn Start() void {
    }
}

impl extends Device {
    fn Start() void {
    }
}
```

The two declarations conflict even when they are in different files.

---

## 6. Members allowed in `impl`

Subject to the specialized rulebooks, an implementation may contain:

- instance methods;
- properties;
- static methods;
- static properties;
- static storage/value declarations;
- immutable associated value declarations using direct `let`;
- lifecycle `init` declarations;
- lifecycle `free` declarations;
- associated/nested type declarations;
- nested implementations of owned associated types;
- unit metadata or other associated declarations explicitly permitted by the
  target type's rulebook.

Examples of associated/nested type declarations include:

```sec
impl Vehicle {
    type Engine struct {
        MaxPower: int,
    }

    enum FuelType {
        Petrol,
        Diesel,
        Electric,
    }

    type Fuel union {
        Petrol
        Diesel
        Electric
    }

    type Flags register[8] {
        Enabled: bit,
        _: bit[7],
    }
}
```

These declarations become members of the implemented type's namespace.

---

## 7. Stored representation must not change

An `impl` must not directly add or redefine the stored representation of its
target.

Forbidden directly in the implementation of the target include:

- stored instance fields;
- register fields of the target;
- enum members of the target;
- union variants of the target;
- any other representation-changing member;
- executable statements outside a member body;
- ordinary local-style `let` declarations directly in the `impl` body.

This remains true across `impl extends` fragments.

Associated nested type definitions are permitted because their representation
belongs to the nested type, not to the outer implementation target.

---

## 8. `self`

Ordinary instance methods have compiler-provided implicit `self`.

Canonical methods do not declare `self` in the parameter list.

```sec
impl Counter {
    fn Increment() void {
        self.value += 1
    }

    fn Value() int {
        return self.value
    }
}
```

The old forms are not canonical:

```sec
fn Read(ref self) int
fn Update(ref mut self, value: int) void
```

Receiver borrowing and mutation requirements are derived and validated by the
compiler from the method body, calls, ownership rules, and borrowing rules.
They are not written as receiver bureaucracy in the source signature.

An ordinary instance method may use `self` because the implementation target is
known from its enclosing `impl`.

Static members have no instance receiver and must not use `self`.

```sec
impl Counter {
    static fn Maximum() int {
        return 100
    }
}
```

`static` remains explicit. Sec does not infer a static member merely because the
member body does not happen to use `self`.

---

## 9. Associated and nested declarations

A nested declaration belongs to the target type's member namespace.

Outside the owning implementation context, it is referenced through the owner:

```sec
Vehicle.Engine
Vehicle.FuelType
```

Inside the primary implementation or a valid same-module extension, the short
name may be used when unambiguous:

```sec
impl Vehicle {
    fn DefaultFuel() FuelType {
        return FuelType.Electric
    }
}
```

A nested declaration must not leak an unqualified name into the surrounding
module namespace.

The complete nested declaration surface must be registered before member bodies
are analyzed so forward references are deterministic.

---

## 10. Nested `impl`

An implementation may contain the primary implementation of a nested/associated
type owned by the enclosing type.

```sec
impl Vehicle {
    type Engine struct {
        MaxPower: int,
    }

    impl Engine {
        fn Start() void {
            // self is Vehicle.Engine here.
        }
    }
}
```

The nested `impl Engine` above is the primary implementation of
`Vehicle.Engine`.

A nested implementation must target a type owned by the enclosing type. It must
not be used merely as a grouping mechanism for an unrelated module-level type.

Invalid:

```sec
type Engine struct {
    MaxPower: int,
}

impl Vehicle {
    impl Engine {
        // Invalid: module-level Engine is not owned by Vehicle.
    }
}
```

Additional fragments for the nested type may use the normal qualified form in
the same module:

```sec
impl extends Vehicle.Engine {
    fn Stop() void {
        // ...
    }
}
```

The same primary/extension, duplicate, module-ownership, and member rules apply
to nested implementations.

---

## 11. Methods

A method declared in an implementation is associated with the target type.

Conceptually:

```sec
impl Vehicle {
    fn Start() void {
    }
}
```

registers a member equivalent to the qualified identity:

```text
Vehicle.Start
```

Methods may be overloaded when the function rules permit it.

Overload identity is based on the parameter signature. Return type does not
participate in overload resolution.

The complete overload/member surface is collected across the primary
implementation and all extension fragments before bodies are analyzed.

Detailed function semantics belong to `functions.md`.

---

## 12. Properties

Properties are behavioral members and belong in `impl`.

```sec
impl Vehicle {
    property TopSpeed: Speed {
        get {
            return self.__topSpeed
        }
    }
}
```

Properties do not add hidden stored instance fields unless a separate property
rule explicitly defines compiler-owned storage semantics for that construct.
Ordinary storage remains declared by the type.

Properties may occur in both the primary `impl Type` and same-module
`impl extends Type` fragments. Those fragments contribute to one combined
member surface, so property duplicates and member conflicts are checked across
all fragments. Instance property bodies receive implicit `self`; receiver
access requirements are inferred from their accessor bodies.

Getter, setter, `try set`, property assignment, and property error semantics are
defined by `properties.md`.

---

## 13. Static members

Type-associated members use explicit `static` where required by `static.md`.
An immutable associated value is the deliberate exception: direct `let` is
its canonical implementation-member spelling.

```sec
impl Counter {
    let Maximum: int := 100

    static fn IsValid(value: int) bool {
        return value <= Counter.Maximum
    }
}
```

Static members do not contribute to instance layout.

Static members do not receive `self`.

For immutable implementation members, `static let Maximum := value` is accepted
as semantically identical compatibility syntax for `let Maximum := value`.
Both spellings register one type-associated immutable member category and use
the same qualified access, initialization, visibility, lifetime, duplicate,
and lowering rules. Canonical formatting removes the redundant `static`.

This equivalence does not extend to mutation. Shared mutable type storage must
be declared `static let mut`; bare `let mut` directly inside an implementation
is invalid. A bare implementation `let` never declares an instance field.

Static storage, methods, and properties may occur in either the primary
implementation or a same-module `impl extends` fragment. All fragments
contribute to one combined static member surface, and duplicate or conflicting
members are rejected across that complete surface.

The combined-surface check treats `let Name` and `static let Name` as the same
member category and identity. Mixing the two spellings cannot evade a duplicate
diagnostic.

Static members are accessed through the target type, never through an
instance. An ordinary `fn` remains instance-bound with implicit `self`; only
`static fn` declares a receiver-free type-level function. A named static
factory is not lifecycle `init`, and `new Type(...)` never selects it.

---

## 14. Lifecycle model

Sec has two special implementation members for explicit lifecycle behavior:

```sec
init
free
```

Neither is declared with `fn`.

They are lifecycle members rather than ordinary callable methods.

`init` establishes a fully valid instance.

`free` performs type-specific cleanup that cannot be handled by compiler-derived
destruction of owned fields.

The lifecycle model does not require either member when ordinary construction
and compiler-derived destruction are sufficient.

---

## 15. `init`

### 15.1 Infallible initializer

An infallible initializer has no return type or error type:

```sec
impl Point {
    init(x: int, y: int) {
        self.x = x
        self.y = y
    }
}
```

### 15.2 Fallible initializer

A fallible initializer writes only its error type after the parameter list:

```sec
impl Connection {
    init(address: Address) ConnectError {
        // ...
    }
}
```

The trailing type is **not a return type**.

It declares only the construction error channel.

A successful initializer does not return a separate success value. Success is
the completed instance itself.

For control-flow and error-propagation analysis, a fallible initializer has an
implicit no-success-payload error channel of the declared error type. It may use
ordinary Sec `try`/error propagation consistent with that exact error type. It
must not return an arbitrary success value.

### 15.3 `init` is not `fn`

Invalid:

```sec
fn init(value: int) void {
}
```

when the programmer intends the lifecycle initializer.

A function or method may still be named `init` where ordinary identifier rules
permit it, but it is an ordinary function and is not selected by `new`.

The bare `init(...)` member form is contextual to `impl`.

### 15.4 Overloading

`init` may be overloaded by parameter signature:

```sec
impl Buffer {
    init() {
    }

    init(size: uint) AllocationError {
    }

    init(data: ref byte[]) AllocationError {
    }
}
```

The trailing error type does not participate in overload identity.

Invalid:

```sec
impl Resource {
    init(path: string) IOError {
    }

    init(path: string) ParseError {
    }
}
```

The two declarations have the same parameter signature and therefore conflict.

### 15.5 Completion requirement

A successful `init` must establish a fully valid instance of the implementation
target.

The instance must satisfy:

- required field initialization;
- type contracts and invariants;
- ownership rules;
- resource ownership rules;
- any type-family-specific validity requirements.

A partially initialized instance must not escape.

### 15.6 Implicit construction path

A type whose ordinary/default construction rules already provide an unambiguous
valid zero-argument construction path does not need to declare `init()` merely
to reproduce that behavior.

Where permitted by the target type's construction rules, `new Type()` may use
that implicit construction path when no explicit matching `init()` is declared.

An explicit matching `init` defines the lifecycle-construction path for that
signature.

### 15.7 Best practice

Do not declare `init` merely to wrap ordinary literal/default construction with
no added semantics.

Use `init` when construction:

- establishes non-trivial invariants;
- performs meaningful initialization logic;
- selects among meaningful construction algorithms;
- acquires resources;
- may fail for a meaningful typed reason.

Do not invent invalid or sentinel states merely to make an initializer
infallible.

---

## 16. `new`

`new` explicitly selects lifecycle construction through `init` or a permitted
implicit construction path.

Infallible:

```sec
let point := new Point(10, 20)
```

Fallible:

```sec
let connection := try new Connection(address)
```

The type of a successful `new Type(...)` expression is always `Type`.

For example:

```sec
let mut connection := try new Connection(address)
```

binds:

```text
connection: Connection
```

It does not bind the error type declared by `init`.

### 16.1 `new` versus conversion

`new` keeps lifecycle construction separate from conversion/casting syntax.

```sec
let percent := Percent(50)
```

uses the type's conversion semantics.

```sec
let value := new SomeType(arguments)
```

uses lifecycle construction.

The two forms must not silently substitute for each other.

### 16.2 `new` does not imply heap allocation

`new` is a constructor-selection marker.

It does **not** mean heap allocation.

The compiler may place the resulting value in stack storage, registers, SSA,
inline aggregate storage, or another valid location according to ordinary
storage and optimization rules.

Allocation occurs only when the selected construction semantics actually
require allocation.

This distinction is normative.

### 16.3 Fallibility

If the selected initializer is fallible, construction requires ordinary Sec
fallibility handling such as `try`.

If the selected initializer is infallible, `try new` is unnecessary and should
be diagnosed according to the ordinary meaningless/redundant-try rules.

---

## 17. Resource acquisition during `init`

Resources acquired during construction must have a statically defined cleanup
path.

For every resource acquired by an initializer, the compiler/analysis pipeline
must be able to determine how the resource is released:

- on every construction-failure path after acquisition; and
- during eventual destruction of a successfully constructed value.

This does not imply that every resource-acquiring type must write a custom
`free` block.

If ownership is represented entirely by owned Sec fields whose types already
have correct destruction, compiler-derived field destruction is sufficient.

A custom `free` is needed only when ownership cannot otherwise be expressed and
released through normal field destruction, for example a foreign resource held
through a non-owning raw representation.

Missing or incompatible cleanup is a compile-time analysis error when it can be
proven from the construction/ownership contract.

---

## 18. Partial initialization and failure

During `init`, fields/resources may become initialized in stages.

If construction fails:

- only successfully initialized owned fields/resources are cleaned up;
- cleanup follows the ownership/destruction rules;
- the incomplete outer value does not escape;
- the completed-value custom `free` operation is not invoked for a value that
  never completed construction;
- explicitly registered construction temporaries are cleaned up as required.

This aligns lifecycle construction with the general partial-construction rules
in `destruction.txt`.

---

## 19. `free`

`free` is the custom lifecycle cleanup member:

```sec
impl ForeignBuffer {
    free {
        unsafe {
            ForeignRelease(self.pointer)
        }
    }
}
```

`free`:

- is not declared with `fn`;
- has compiler-provided `self` destruction access;
- has no ordinary return value;
- is not an ordinary method and is not called as `value.free()`;
- must not allow the complete value to escape or be resurrected;
- is followed by automatic destruction of remaining initialized owned fields
  according to the destruction rules;
- exists at most once for the complete merged implementation of a type.

Detailed destruction, cleanup ordering, partial-move restrictions, and
deallocation rules are defined by `destruction.txt`.

---

## 20. Constructor/destructor balance

`init` and `free` form the explicit lifecycle pair, but they are not required to
appear together textually.

The semantic requirement is stronger and more useful than textual symmetry:

> Every owned resource established during construction must have a valid cleanup
> path for both failed construction and successful lifetime completion.

Compiler-derived destruction may satisfy the successful cleanup side without a
custom `free`.

When custom cleanup is necessary, it must be defined no later than the type's
`free` lifecycle operation or by another ownership mechanism explicitly defined
by the memory rules.

The compiler's ownership, destruction, escape, effect, and resource analyses
should verify this relationship where statically possible.

---

## 21. Generic implementations

A generic implementation uses the generic parameters of its target type.

```sec
type Holder[T] struct {
    value: T,
}

impl Holder[T] {
    fn Get() T {
        return self.value
    }
}
```

The generic target arguments must correspond to the target declaration's
generic parameters according to `generics.md`.

The same rule applies to `impl extends Holder[T]`. The primary and all valid
same-module extensions compose into one concrete member surface for each
monomorphized `Holder[T]`.

Methods, including static methods, may introduce additional method-level
generic parameters. A method inside `impl Holder[T]` sees both the owning `T`
and its own parameters, such as `fn Transform[U](...) Holder[U]`. Method-level
generic parameters are canonical Sec 0.1 behavior and are not postponed.
Concrete instance methods continue to use implicit `self`.

---

## 22. Name lookup and member registration

The compiler must collect the complete member surface before analyzing member
bodies.

At minimum, this includes:

1. primary implementation ownership;
2. valid `impl extends` fragments;
3. associated/nested type names;
4. nested implementation targets;
5. properties;
6. method overload groups;
7. static members;
8. `init` overloads;
9. `free` presence;
10. other associated members permitted by specialized rulebooks.

This permits deterministic forward references and duplicate diagnostics across
files.

The primary implementation and all extensions contribute to one member
namespace.

---

## 23. Duplicate and conflict rules

The combined implementation must reject:

- multiple primary implementations for one target;
- an extension with no primary implementation;
- an extension outside the defining module;
- an ordinary primary implementation outside the type's defining module;
- duplicate non-overloadable member names;
- duplicate method signatures;
- duplicate `init` parameter signatures;
- more than one `free`;
- conflicts between a field and a behavior/associated member;
- conflicts between associated/nested type names and other members;
- nested implementations of unrelated types;
- representation-changing declarations directly in the outer `impl`.

Diagnostics should identify both declarations when a conflict has two source
sites.

---

## 24. Interface boundary

Ordinary `impl` structure and interface conformance are separate semantic
concerns, but explicit conformance is declared at the primary impl entry point:

```sec
impl FileReader implements Reader, Closeable {
}
```

An `impl extends FileReader` fragment contributes members but cannot redeclare
the interface list.

This rulebook defines only the ordinary implementation surface.

The interface rulebook defines:

- how a type declares conformance;
- the compatibility semantics of the primary impl's `implements` list;
- requirement matching;
- interface coherence rules.

An interface implementation must not create a second ordinary primary
implementation for the type and must not change the type's stored
representation.

---

## 25. Runtime and code-generation model

`impl` itself has no runtime representation.

An implementation block does not add:

- a hidden instance field;
- a method table merely because methods exist;
- a vtable unless a separate interface/runtime rule requires one;
- hidden metadata pointers;
- storage solely because a member is declared.

Methods, properties, static members, `init`, `free`, and interface operations are
lowered according to their own semantic requirements.

`new` must not force heap allocation.

---

## 26. Best practices

Rulebooks are normative sources for generated manuals, so these recommendations
are intentionally recorded here.

### 26.1 Primary implementation as entry point

Keep the primary `impl Type` near the type declaration when practical.

Place the most central behavior, lifecycle declarations, and high-value reader
entry points in the primary implementation.

Use `impl extends` to split coherent areas of behavior across files, not to
scatter unrelated methods arbitrarily.

### 26.2 Data/behavior separation

Keep stored instance representation in the type declaration.

Use `impl` for behavior and associated definitions that do not mutate the outer
type's representation.

### 26.3 Construction

Prefer ordinary implicit/literal construction when it already expresses the
required semantics.

Use `init` when construction has meaningful logic, invariants, resource
acquisition, or fallibility.

Use `new` only to request lifecycle construction. Do not read `new` as "heap
allocate".

### 26.4 Destruction

Prefer compiler-derived destruction.

Define `free` only for cleanup not already represented by owned fields.

Do not manually release resources that owned fields will already destroy.

### 26.5 Extensions

Group extension fragments by coherent concern and keep duplicate/member identity
stable across the whole module.

---

## 27. Required diagnostics

Representative required diagnostics include:

```text
duplicate primary impl for Device; additional blocks must use impl extends Device
```

```text
impl extends Device requires a primary impl Device in the same module
```

```text
ordinary impl for graphics.Image must be declared in module graphics
```

```text
impl methods have implicit self; remove self from the parameter list
```

```text
static member Reset cannot use self
```

```text
stored instance fields are not allowed inside impl Device
```

```text
nested impl target Engine is not owned by Vehicle
```

```text
duplicate init signature init(path: string) for Resource
```

```text
new Connection(address) selects fallible init; use try or handle ConnectError
```

```text
construction of ForeignBuffer acquires a resource with no provable cleanup path
```

```text
free is a lifecycle member and cannot be called directly
```

---

## 28. Tooling requirements

The lexer, parser, formatter, LSP, syntax highlighter, AST printer, diagnostics,
and compiler analyses must agree on the canonical forms in this rulebook.

In particular:

- `new` is a hard language keyword;
- `init` is a contextual lifecycle-member spelling inside `impl`;
- `impl extends Type` is canonical;
- explicit `self` parameters are non-canonical and rejected by Sema;
- hover/signature help for `init` must distinguish the construction error type
  from a return type;
- hover for `new Type(...)` should report the constructed type and, when
  applicable, the construction error type;
- formatter output must preserve the distinction between conversion
  `Type(value)` and lifecycle construction `new Type(args...)`.

Suggested LSP rendering:

```text
init(address: Address)
constructs: Connection
construction error: ConnectError
```

---

## 29. Related rulebooks

Detailed semantics are owned by the narrow rulebooks where applicable:

- `functions.md` — function and method signatures, bodies, overloads;
- `properties.md` — properties and fallible setters;
- `static.md` — static storage and static members;
- `generics.md` — generic targets and parameters;
- `interfaces.md` — interface conformance and interface implementations;
- `destruction.txt` — destruction, partial construction, and `free` cleanup;
- `errorhandling.md` — `try`, error propagation, and exact error typing;
- `ownership.md` / `borrowing.md` — ownership and receiver/body access;
- `names_scopes_visibility.md` — member namespaces, overload identity, visibility;
- `grammar.md` — consolidated syntax;
- `lexical_structure.md` — reserved words including `new`.

---

## 30. Version 2.0 semantic delta

Revision 2.0 intentionally changes and consolidates earlier `impl.txt` material.

Major deltas include:

- canonical implicit `self`; no `ref self` / `ref mut self` receiver syntax in
  canonical methods;
- exactly one human-facing primary `impl Type` plus same-module
  `impl extends Type` fragments;
- explicit defining-module ownership for ordinary implementations;
- associated/nested declarations are permitted so long as they do not change
  the outer target's stored representation;
- nested implementations are permitted for nested/associated types owned by the
  enclosing type;
- `init` is standardized as a special lifecycle member;
- fallible `init(args...) ErrorType` uses a construction error type, not a
  return type;
- `free` is the matching explicit cleanup lifecycle member where compiler-derived
  destruction is insufficient;
- `new Type(args...)` is introduced to select lifecycle construction without
  colliding with `Type(value)` conversion syntax;
- `new` is a newly reserved hard keyword and therefore a source-compatibility
  change.
