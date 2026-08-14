# Sec Generics

- **Status:** Normative
- **Created:** 2026-08-14
- **Last updated:** 2026-08-14
- **Document revision:** 2.0
- **Language version:** Sec 0.1
- **Replaces:** `rules/declarations/generics.txt`
- **Canonical path:** `rules/declarations/generics.md`

## 1. Purpose

Generics allow declarations to be written once and instantiated with concrete types during compilation.

Generics are a compile-time language feature.

Generic type parameters do not exist as runtime values.

Sec generics are monomorphized. Every reachable concrete combination of generic arguments produces or reuses a concrete specialized instantiation that is type-checked and lowered as ordinary concrete code.

The generic model preserves:

- static typing;
- nominal type identity;
- explicit interface constraints;
- no runtime generic dispatch;
- no type erasure;
- no implicit boxing;
- no hidden allocation;
- no runtime reflection requirement;
- no runtime representation of generic type parameters;
- predictable concrete layout;
- deterministic symbol identity;
- understandable diagnostics.

## 2. Generic parameter syntax

Generic parameters use square brackets after the declaration name.

```sec
type Pair[A, B] struct {
    first: A,
    second: B,
}
```

```sec
fn Identity[T](value: T) T {
    return value
}
```

Parameter names are type names within the generic declaration's scope.

Generic parameter names must be unique within one parameter list.

Parameter order is significant.

Trailing commas are permitted where ordinary generic-list grammar permits them.

## 3. Generic arguments

Generic arguments also use square brackets.

```sec
Pair[string, int]
Option[User]
Identity[int](10)
```

Generic arguments are types.

Generic arguments are not arbitrary runtime expressions.

A generic type reference must provide the complete generic argument list.

```sec
Pair[string, int]
```

A generic function or method call may infer missing generic arguments according to the inference rules in this rulebook.

## 4. Type identity

Generic instantiations preserve ordinary Sec type identity.

These refer to the same concrete type:

```sec
Pair[string, int]
Pair[string, int]
```

These refer to different concrete types:

```sec
Pair[string, int]
Pair[int, string]
```

Nominal generic declarations remain nominal.

Two unrelated declarations do not become the same type merely because their substituted representations match.

Named types, units, structs, unions, interfaces and other valid concrete Sec types preserve their normal identity when used as generic arguments.

## 5. Supported generic declaration families

Sec 0.1 supports generic parameters on:

- structs;
- unions;
- named types;
- interfaces;
- functions;
- methods;
- eligible implementation targets through their generic type;
- nested declarations where the owning declaration and normal scope rules permit them.

### 5.1 Generic struct

```sec
type Stack[T] struct {
    items: list[T],
}
```

### 5.2 Generic union

```sec
type Option[T] union {
    Some(T)
    None
}
```

### 5.3 Generic named type

```sec
type ID[T] int
```

For example:

```sec
ID[User]
ID[Product]
```

are distinct concrete nominal types.

Normal named-type conversion, contract, copy/move and representation rules apply after substitution.

A generic named type may also use its type parameter as the represented type when permitted by the ordinary type rules.

```sec
type Wrapped[T] T
```

### 5.4 Generic interface

```sec
interface Repository[T] {
    fn Get(id: ID[T]) Option[T]
}
```

A concrete generic interface instantiation is a distinct interface type.

```sec
Repository[User]
Repository[Product]
```

### 5.5 Generic function

```sec
fn Convert[From, To](value: From) To {
    ...
}
```

### 5.6 Generic method

Methods may declare additional generic parameters.

```sec
impl Stack[T] {
    fn Map[U](mapper: fn(T) U) Stack[U] {
        ...
    }
}
```

The method sees both the generic parameters of the owning generic type and its own method-level parameters.

Method-level generics are part of Sec 0.1 and are not postponed.

## 6. Generic enums

Enums do not accept generic parameters.

Invalid:

```sec
enum Value[T] {
    First
    Second
}
```

Expected diagnostic:

```text
enum declarations cannot have generic parameters
```

An enum represents a finite set of named value classes. Generic payload variation belongs in a union rather than in an enum.

## 7. Generic registers

This rulebook does not by itself authorize arbitrary generic register declarations.

Register representation must always have a statically determined fixed width and valid field layout.

A register-specific rule may permit a generic register form only when every concrete instantiation has a representation that can be proven valid under the register rules.

Generic parameters must never make register width, field boundaries, bit allocation or byte representation indeterminate at layout time.

## 8. Generic impl blocks

A generic nominal type uses its type parameters in the target of its implementation.

```sec
type Stack[T] struct {
    items: list[T],
}

impl Stack[T] {
    fn Push(value: T) void {
        self.items.Append(value)
    }
}
```

The parameters in the impl target correspond to the parameters declared by the target type.

They are not unrelated implicit generic parameters.

Invalid:

```sec
type Stack[T] struct {
}

impl Stack[U] {
}
```

if `U` is not the corresponding declared target parameter.

The primary implementation follows the ordinary `impl` rules.

Additional same-module behavior may be declared through `impl extends`.

```sec
impl extends Stack[T] {
    fn IsEmpty() bool {
        return self.items.IsEmpty
    }
}
```

Generic primary and extended implementation fragments contribute to the same concrete implementation surface after substitution.

## 9. Method-level generic parameters

A method may introduce generic parameters independently of the owning type.

```sec
impl Converter {
    fn Convert[T, U](value: T) U {
        ...
    }
}
```

A method inside a generic implementation may combine outer and method-level parameters.

```sec
impl Box[T] {
    fn Transform[U](transform: fn(T) U) Box[U] {
        ...
    }
}
```

The scopes are nested:

- `T` belongs to the generic type/impl;
- `U` belongs to the method.

A method-level parameter may not redeclare a visible generic parameter name from an enclosing generic scope.

Static methods may also declare method-level generic parameters.

## 10. Generic methods in interface contracts

A generic interface may use the interface's own generic parameters in ordinary method requirements.

```sec
interface Sink[T] {
    mut fn Write(value: T) void
}
```

An interface method requirement does not introduce additional method-level generic parameters.

Invalid:

```sec
interface Mapper[T] {
    fn Map[U](value: T) U
}
```

Reason: interface values may require runtime dispatch, while Sec explicitly has no runtime generic dispatch. The dispatch surface of an interface must therefore be finite after the interface itself is concretely instantiated.

Use a generic interface parameter, another interface, or a generic concrete helper when the operation itself must vary by another type.

## 11. Constraints

Generic constraints are compile-time requirements expressed after `:`.

```sec
fn Save[T: Serializable](value: T) Result[void, IOError] {
    return value.Serialize()
}
```

`T: Serializable` does not declare interface implementation.

It states that any concrete type substituted for `T` must already satisfy the `Serializable` interface constraint.

Explicit interface conformance remains declared separately on the concrete type's primary implementation.

```sec
impl User implements Serializable {
    fn Serialize() byte[] {
        ...
    }
}
```

A matching method name without valid explicit conformance is not sufficient.

## 12. Multiple constraints

Multiple interface constraints use `&`.

```sec
fn Save[T: Serializable & Comparable](value: T) void {
    ...
}
```

All listed constraints must be satisfied.

More than two constraints compose in the same way.

```sec
fn Process[T: Serializable & Comparable & Printable](value: T) void {
    ...
}
```

`&` means logical conjunction of generic constraints.

It does not create a runtime intersection type.

Comma separates generic parameters, not constraints.

```sec
fn Convert[
    T: Serializable & Comparable,
    U: Printable,
](value: T) U {
    ...
}
```

## 13. Constraint resolution

Each constraint must resolve to an interface type or a valid concrete generic interface instantiation.

Valid:

```sec
fn Save[T: Serializable](value: T) void {
    ...
}
```

Valid:

```sec
fn Compare[T: Comparable[T]](left: T, right: T) int {
    ...
}
```

Invalid:

```sec
fn Save[T: int](value: T) void {
    ...
}
```

Expected diagnostic:

```text
generic constraint int is not an interface
```

Unknown constraints are invalid.

```text
unknown generic constraint Missing for T
```

## 14. Constraint satisfaction

A concrete type satisfies a generic interface constraint only through valid interface conformance.

```sec
interface Serializable {
    fn Serialize() byte[]
}

type User struct {
    ...
}

impl User implements Serializable {
    fn Serialize() byte[] {
        ...
    }
}
```

Then:

```sec
Save[User](user)
```

may satisfy `T: Serializable`.

Constraints do not:

- create runtime interface values;
- imply dynamic dispatch;
- automatically implement interfaces;
- infer structural conformance from matching member names.

## 15. Operations available on generic parameters

A generic declaration is checked against the guarantees provided by its parameters and constraints.

An unconstrained parameter supports only operations that are valid without assuming additional capabilities.

A constrained parameter may use members guaranteed by its constraints.

```sec
fn Encode[T: Serializable](value: T) byte[] {
    return value.Serialize()
}
```

The generic body must not use extra members merely because one concrete instantiation happens to provide them.

This keeps generic semantics independent of accidental properties of particular call sites.

Concrete-dependent checks such as final layout, defaultability and copy/move classification are still performed after substitution where required.

## 16. Ownership, copying and borrowing

Generic parameters do not imply copyability.

```sec
fn Duplicate[T](value: T) Pair[T, T] {
    return Pair[T, T] {
        first: value,
        second: value,
    }
}
```

This operation is not valid unless the generic contract guarantees that using `value` twice is legal.

Generic code follows the normal:

- ownership rules;
- move rules;
- copy rules;
- borrowing rules;
- mutability rules;
- destruction rules.

The compiler must never implement generic operations by blindly copying runtime bytes.

Concrete instantiations receive their final copy/move/destruction classification after substitution.

Generic functions may use ordinary, borrowed, forced-consuming, and native
typed variadic parameters:

```sec
fn Store[T](-> value: T) void {
    ...
}

fn Count[T](values: ...T) int {
    return values.Len
}
```

The consuming contract remains unconditional after substitution, including
when the concrete `T` is copyable. Each variadic instantiation has one concrete
element type. `-> values: ...T` is invalid. Monomorphization preserves both
parameter ownership mode and variadic shape in the concrete callable identity.

## 17. Defaultability

A generic parameter has no implicit defaultability guarantee.

Invalid for unconstrained `T`:

```sec
fn Make[T]() T {
    let mut value: T
    return value
}
```

Generic code may use default construction only when the generic contract guarantees it under the applicable type/interface rules.

A concrete instantiation being Defaultable does not retroactively make an otherwise invalid unconstrained generic template valid.

## 18. Generic function inference

Generic function and method arguments may be inferred from:

- ordinary function arguments;
- receiver and owning generic type;
- expected result type where the surrounding expression supplies one;
- already explicit generic arguments.

Inference must resolve one consistent concrete type for each unresolved generic parameter.

Example:

```sec
let value := Identity(10)
```

may infer `T` from the argument.

Expected-result context may participate when necessary.

```sec
let result: Target := Convert(source)
```

may use the expected result type to infer a result-related generic parameter when the signature provides that relationship.

If inference cannot uniquely determine a parameter, the call is invalid and the programmer must supply more generic arguments or ordinary type context.

## 19. Explicit generic arguments

A caller may provide all generic arguments explicitly.

```sec
let value := Identity[int](10)
```

Explicit generic arguments are applied before inference.

They must satisfy:

- generic arity/prefix rules;
- type validity;
- constraints;
- ordinary parameter compatibility.

## 20. Partial explicit generic arguments

A function or method call may provide a positional prefix of its generic arguments and infer the remainder.

```sec
fn Convert[From, To](value: From) To {
    ...
}
```

Valid:

```sec
let result: string := Convert[int](value)
```

Here:

- `From` is explicitly fixed to `int`;
- `To` is inferred from remaining call/result context.

Also valid:

```sec
Foo[A](...)
Foo[A, B](...)
```

when the declaration has further parameters that can be inferred.

Explicit arguments always bind from left to right.

Generic argument holes are not supported.

Invalid:

```sec
Foo[, B](...)
```

Invalid:

```sec
Foo[_, B](...)
```

unless `_` is separately defined as generic inference syntax by another normative rule.

A generic type reference does not use partial inference; type references require all generic arguments.

## 21. Generic static members

Static storage in a generic implementation is specialized per concrete generic instantiation.

```sec
impl Cache[T] {
    static let mut Count: int := 0
}
```

These are distinct storage locations:

```sec
Cache[int].Count
Cache[string].Count
```

Static methods and static properties are likewise resolved on the concrete specialized type.

No hidden storage is shared across all `T` merely because one source template exists.

## 22. Generic interfaces

A generic interface is instantiated with concrete generic arguments before use as a concrete interface type or constraint.

```sec
interface Serializer[T] {
    fn Serialize(value: ref T) byte[]
}
```

Examples:

```sec
Serializer[User]
Serializer[Product]
```

are distinct interface instantiations.

Conformance must match the concrete instantiated interface contract.

Interface inheritance may refer to generic interface instantiations according to the interface rules.

## 23. Generic unions

Generic union variants are substituted with concrete types before layout, construction, matching and destruction.

```sec
type Result[T, E] union {
    Ok(T)
    Err(E)
}
```

For:

```sec
Result[int, IOError]
```

the compiler operates on a concrete closed union with concrete variant payload types.

Exhaustiveness and payload ownership are checked on the substituted concrete union.

## 24. Recursive generic types

Generic recursion must terminate in a finite concrete representation.

Direct infinite storage is invalid.

```sec
type Node[T] struct {
    next: Node[T],
}
```

unless the recursion is broken by a valid indirection/reference/container whose own rules provide finite representation.

Changing generic arguments must not create non-converging instantiation.

The compiler must detect recursive-instantiation cycles and report the instantiation path.

## 25. Monomorphization

Generics are monomorphized before backend code generation.

A concrete instantiation includes the complete substitution of all generic parameters.

For every reachable concrete type instance, the compiler must know:

- concrete field/variant representation;
- size and alignment;
- defaultability;
- copy/move classification;
- destruction behavior;
- implemented interfaces;
- concrete methods;
- concrete static storage identity.

For every reachable concrete function/method instance, the compiler must know:

- concrete parameter types;
- concrete result type;
- concrete constraints already satisfied;
- concrete called symbols;
- concrete ownership behavior.

Unresolved generic parameters must not reach ABI lowering, MLIR lowering that requires concrete representation, or LLVM code generation.

## 26. Lazy instantiation

Canonical behavior is demand-driven monomorphization.

Unused concrete generic functions and methods need not produce backend code.

Repeated requests for the same declaration with the same concrete generic arguments reuse one canonical concrete instance.

The compiler may analyze generic templates before concrete use, but concrete code generation is based on reachable concrete instantiations.

## 27. ABI boundary

Generic source declarations do not themselves define a runtime ABI.

ABI sees concrete monomorphized instances.

For example:

```sec
fn Encode[T](value: T) byte[] {
    ...
}
```

has no single unresolved runtime ABI as `Encode[T]`.

A concrete reachable instantiation such as:

```sec
Encode[Packet]
```

may have an ABI once `Packet` and the full concrete function signature are known.

Foreign/exported ABI rules may impose additional restrictions on which concrete generic instantiations are ABI-visible.

Those restrictions belong to the ABI/FFI rulebooks.

## 28. Function values

A generic function or method declaration is a compile-time template, not one polymorphic runtime function value.

A function value must refer to a concrete specialized callable.

```sec
let transform: fn(int) string := Convert[int, string]
```

where otherwise valid.

Sec does not represent an unresolved generic function template as a runtime callable object.

Generic lambda templates are not part of Sec 0.1. An anonymous
`fn[T](value: T) T { ... }` would be an unresolved compile-time template rather
than one concrete runtime callable. Use a named generic function and concretely
specialize it before converting it to a function value.

## 29. Overload resolution

Generic resolution participates in ordinary overload resolution.

The compiler must consider:

- ordinary parameter compatibility;
- explicit generic prefix arguments;
- inferred generic arguments;
- constraint satisfaction;
- expected result context where permitted.

Return type alone does not create otherwise duplicate overload declarations.

Inference must not select a candidate by inventing conversions that ordinary overload rules would reject.

## 30. Generic parameter scope

Generic parameters are type symbols.

They are visible only in the declaration scopes that own them.

For a generic type:

```sec
type Box[T] struct {
    value: T,
}
```

`T` is visible in the type declaration and its matching generic implementation scopes.

Method-level generic parameters are visible only in that method and nested scopes.

Generic parameters may shadow unrelated outer names only where normal type-symbol shadowing rules permit it.

A nested generic parameter may not redeclare another generic parameter from the same active generic declaration chain when that would make substitution ambiguous.

## 31. Unsupported generic mechanisms

The following are not part of the Sec 0.1 generic model:

- runtime generic dispatch;
- type erasure;
- implicit boxing;
- higher-kinded types;
- generic parameter packs;
- variadic type-parameter lists;
- generic enums;
- generic argument holes;
- automatic structural constraints inferred from member names;
- automatic interface implementation;
- negative constraints;
- union-of-types constraints;
- arbitrary compile-time reflection as a generic mechanism.

Const/value generics are not introduced by this rulebook. Compile-time value parameterization, if present elsewhere in Sec, must have separate normative semantics and must not be inferred from type-generic syntax.

Default generic type arguments are not part of the Sec 0.1 generic syntax.

## 32. Parser requirements

The parser must support:

- generic parameter lists after eligible declaration names;
- generic method parameter lists;
- `:` interface constraints;
- `&` between multiple constraints;
- comma-separated generic parameters;
- trailing commas;
- generic arguments in type references;
- generic arguments on function/method calls;
- partial explicit function/method generic prefixes;
- nested generic type references;
- generic interface declarations;
- generic named type declarations;
- generic impl targets.
- ordinary, borrowed, forced-consuming, and typed variadic parameters on generic functions and methods.

The parser must reject or preserve for Sema with clear source locations:

- malformed generic lists;
- missing constraint after `:`;
- missing constraint after `&`;
- generic parameters on enums;
- generic argument holes;
- arbitrary runtime expressions used as type arguments.

## 33. Sema requirements

Sema must:

- register generic declarations as templates;
- register generic arity before body analysis;
- register generic parameters as type symbols;
- reject duplicate generic parameter names;
- resolve all interface constraints;
- support multiple `&` constraints;
- reject unknown constraints;
- reject constraints that are not interfaces;
- verify explicit interface conformance for constraint satisfaction;
- create generic scopes;
- validate generic impl parameter correspondence;
- support method-level generic parameters;
- infer missing function/method generic arguments;
- apply explicit positional-prefix arguments before inference;
- reject unresolved generic parameters;
- reject generic argument holes;
- check constraints after resolution;
- canonicalize identical concrete instantiations;
- detect infinite recursive instantiation;
- substitute concrete types recursively;
- re-evaluate concrete layout/default/copy/move/destruction properties;
- preserve nominal identity;
- preserve consuming parameter modes and typed variadic shape through every concrete callable instantiation;
- register concrete method and static-member surfaces;
- ensure generic bodies use only capabilities guaranteed by their constraints;
- reject unresolved generic representation before ABI/backend lowering;
- report template-independent errors once;
- report concrete-dependent failures with instantiation context.

## 34. Semantic IR requirements

Semantic IR must distinguish generic templates from concrete instantiations.

It must preserve enough information to identify:

- source generic declaration;
- ordered generic parameters;
- constraints;
- concrete generic arguments;
- canonical concrete instance identity;
- concrete substituted types;
- concrete called method/function symbols;
- instantiation source location.

Representation-dependent IR must operate on concrete types.

No backend-facing IR node may rely on runtime knowledge of an unresolved type parameter.

## 35. Diagnostics

Required diagnostics include at least:

```text
duplicate generic parameter T
```

```text
unknown generic constraint Missing for T
```

```text
generic constraint int is not an interface
```

```text
type User does not satisfy constraint Serializable for T
```

```text
cannot infer generic parameter To
```

```text
too many explicit generic arguments for Convert
```

```text
generic argument holes are not supported
```

```text
enum declarations cannot have generic parameters
```

```text
method-level generic parameter U conflicts with enclosing generic parameter U
```

```text
generic operation requires capability not guaranteed for T
```

```text
recursive generic instantiation does not produce a finite type
```

Diagnostics for concrete instantiation failures should show both the failing generic declaration and the concrete substitution that triggered the failure.

## 36. Best practice

- Keep generic parameter lists small and semantically meaningful.
- Use constraints only for capabilities the implementation actually needs.
- Prefer `T: Interface` over manual runtime capability checks.
- Combine independent interface requirements with `&`.
- Let inference handle obvious parameters; use an explicit positional prefix when it improves clarity or resolves ambiguity.
- Use method-level generics when the variation belongs to one operation rather than the whole type.
- Use generic unions for payload variation; do not model payload variation with generic enums.
- Avoid exposing unnecessary generic complexity across ABI or FFI boundaries.
- Keep static state in generic types intentional because every concrete specialization receives distinct storage.

## 37. Cross-rulebook ownership

This rulebook owns compile-time type generics, constraints, inference and monomorphization semantics.

Related rules are owned elsewhere:

- concrete `impl`, `impl extends`, implicit `self`, `new` and lifecycle members: `rules/declarations/impl.md`;
- interface declaration and explicit conformance: `rules/declarations/interfaces.md`;
- static members and per-specialization static storage: `rules/declarations/static.md`;
- named-type identity and contracts: type rulebooks;
- struct/union/enum/register representation: their declaration rulebooks;
- ownership, copy/move and borrowing: memory rulebooks;
- function signatures and overload resolution: functions rulebook;
- ABI-visible concrete representation: `rules/platform/abi.md`;
- FFI restrictions: platform FFI rulebook;
- Semantic IR and MLIR lowering: compiler rulebooks.
