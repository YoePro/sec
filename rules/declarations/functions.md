# Sec Functions

- **Status:** Normative
- **Created:** 2026-08-14
- **Last updated:** 2026-08-14
- **Document revision:** 2.0
- **Language version:** Sec 0.1
- **Replaces:** `rules/declarations/functions.txt`
- **Canonical path:** `rules/declarations/functions.md`

## 1. Purpose

This rulebook defines ordinary Sec function declarations, parameters, calls, return values, overloads, consuming parameters, native variadic functions, and the function-level boundaries to methods, interfaces, generics, error handling, ownership, ABI, and FFI.

Ordinary Sec functions are statically typed and have explicit signatures.

## 2. Ordinary function declaration

An ordinary named function is declared with `fn`.

```sec
fn Add(left: int, right: int) int {
    return left + right
}
```

The canonical form is:

```text
fn Name(ParameterList) ReturnType {
    Body
}
```

An ordinary named Sec function always has:

- a name;
- an explicit parameter list;
- an explicit return type;
- a body.

Declaration order does not determine visibility between functions in the same valid declaration scope. Function signatures are registered before bodies are analyzed.

## 3. Explicit return type

Every declaration using ordinary `fn` syntax has an explicit return type.

```sec
fn Calculate(value: int) int {
    return value * 2
}
```

A function with no result uses `void`.

```sec
fn Notify() void {
    return
}
```

Return-type inference is not part of ordinary named function declarations in Sec 0.1.

Invalid:

```sec
fn Calculate(value: int) {
    return value * 2
}
```

Expected diagnostic:

```text
function Calculate must declare an explicit return type
```

### 3.1 Lifecycle members are not `fn`

`init` and `free` are special lifecycle members and are not function declarations using `fn`.

```sec
impl Resource {
    init(size: int) {
        ...
    }

    free {
        ...
    }
}
```

Their syntax and lifecycle semantics are defined by `impl.md` and the destruction rules.

They are therefore not exceptions to the explicit-return-type rule for `fn`.

## 4. No ordinary bodyless prototypes

Ordinary Sec functions require a body.

Invalid as an ordinary Sec function:

```sec
fn Calculate(value: int) int
```

Sec 0.1 has no C/C++-style ordinary function prototype declarations.

Bodyless callable signatures are permitted only where another construct explicitly defines that meaning, including:

- interface method requirements;
- foreign `extern` declarations governed by FFI/ABI rules.

An interface requirement is not an ordinary function definition.

```sec
interface Calculator {
    fn Calculate(value: int) int
}
```

An `extern` declaration is a foreign declaration and is governed separately.

## 5. Parameters

Ordinary parameters use:

```text
name: Type
```

Example:

```sec
fn Distance(x: float64, y: float64) float64 {
    ...
}
```

Parameters are comma-separated.

A trailing comma is allowed.

```sec
fn Mix(
    left: int,
    right: int,
) int {
    ...
}
```

Parameter names must be unique within one function parameter list.

Parameter types must resolve to valid types.

## 6. Owned by-value parameters are mutable local bindings

An ordinary by-value parameter is owned by the callee for the duration of the call.

The parameter binding is mutable by default.

```sec
fn ClampToZero(value: int) int {
    if value < 0 {
        value = 0
    }

    return value
}
```

No `mut` marker is required on an owned by-value parameter.

This is an intentional exception to ordinary immutable-by-default local declarations.

The reason is ownership:

- for an implicitly copyable type, the callee owns its received copy;
- for a move-only type, ownership is transferred to the callee;
- in both cases, the function owns the local parameter value it is allowed to work with.

Mutation of a copied by-value parameter does not modify the caller's original value.

```sec
fn Increment(value: int) int {
    value += 1
    return value
}
```

The caller's original `int` remains unchanged.

For an owned move-only parameter, the caller no longer owns the moved value after successful call entry.

Reassignment of an owned parameter follows normal destruction, ownership, and definite-assignment rules for replacing an owned local value.

## 7. Borrowed parameters

A parameter of type `ref T` is a shared borrow.

```sec
fn Inspect(value: ref Buffer) void {
    ...
}
```

The callee does not own the referred-to value.

The caller retains ownership.

A shared borrow does not permit mutation of the referent.

A parameter of type `ref mut T` is an exclusive mutable borrow.

```sec
fn Modify(value: ref mut Buffer) void {
    ...
}
```

The caller retains ownership of the referred-to value.

The callee receives temporary exclusive mutable authority according to the borrowing rules.

The reference parameter binding itself is not an owned working-value parameter and is not implicitly rebindable merely because owned by-value parameters are mutable.

`ref` and `ref mut` semantics are defined in the borrowing rulebook.

## 8. Ordinary by-value ownership transfer

For an ordinary parameter:

```sec
fn Process(value: T) void {
    ...
}
```

argument transfer follows the concrete type's copy/move classification.

For an implicitly copyable `T`:

```text
caller value -> copied value owned by parameter
caller retains original
```

For a move-only `T`:

```text
caller ownership -> parameter ownership
caller source becomes unavailable after successful call entry
```

The function signature does not need a `move` keyword to express ordinary move-only transfer.

## 9. Explicit consuming parameter

A by-value parameter may explicitly require ownership consumption with `->`.

```sec
fn Transform(-> data: BigArray) BigArray {
    ...
}
```

A consuming parameter transfers ownership of the argument to the callee regardless of whether the concrete type would otherwise be implicitly copied.

For a move-only type, this agrees with ordinary by-value transfer.

For a copyable type, `->` forces consuming transfer instead of ordinary copy semantics.

Example:

```sec
let data := CreateLargeArray()
let result := Transform(data)

Use(data)
```

The final use is invalid because `Transform` consumes `data`.

A consuming parameter is an ownership contract, not a hint about machine-level ABI passing.

The call-site spelling remains an ordinary call:

```sec
Transform(data)
```

The callee signature determines that the argument is consuming.

## 10. Restrictions on consuming parameters

`->` applies to the parameter value itself.

It does not grant ownership of pointees merely because the value contains or represents an address.

For example, consuming a `RawPtr[T]` consumes that pointer value binding; it does not create ownership of the pointed-to storage.

A consuming parameter must not be combined with a borrowed parameter form to imply ownership of a borrowed referent.

Invalid:

```sec
fn Take(-> value: ref Buffer) void {
    ...
}
```

Invalid:

```sec
fn Take(-> value: ref mut Buffer) void {
    ...
}
```

Use an owned parameter when ownership transfer is intended.

## 11. Consuming mode and overload identity

Overloads must not differ only by ordinary versus consuming by-value mode.

Invalid:

```sec
fn Process(value: Buffer) void {
    ...
}

fn Process(-> value: Buffer) void {
    ...
}
```

These declarations have the same overload parameter signature and conflict.

Reason:

```sec
Process(buffer)
```

must not allow overload resolution to decide whether the caller retains or loses ownership.

The ownership mode is part of the callable contract and callable-type compatibility, but it is not a discriminator that permits two otherwise identical overload declarations.

## 12. Return statements

A non-`void` function returns exactly one value.

```sec
fn Answer() int {
    return 42
}
```

The returned expression must be assignable to the declared return type.

Every reachable path that must produce a result must satisfy ordinary return analysis.

A `void` function may use:

```sec
return
```

and may also reach the end of its body when ordinary control-flow rules permit.

Returning an expression from a `void` function is invalid.

## 13. Single return value

Sec 0.1 does not support multiple return values.

Invalid:

```sec
fn Split(value: int) (int, int) {
    ...
}
```

When several related values must be returned, use one explicit type.

```sec
type SplitResult struct {
    left: int,
    right: int,
}

fn Split(value: int) SplitResult {
    ...
}
```

A generic union, named struct, collection, or another single explicit type may be used where appropriate.

## 14. Return ownership

Returning an owned value transfers the returned value to the caller according to the normal copy/move rules.

A move-only local may be returned without creating a duplicate owner.

```sec
fn CreateBuffer() Buffer {
    let buffer := Buffer.Create(4096)
    return buffer
}
```

After successful return, the caller owns the returned value.

Borrowed returns must satisfy the borrowing and lifetime rules.

## 15. Function body scope

A function body creates a lexical scope.

Parameters are declared in that function scope before body statements are analyzed.

Local declarations follow ordinary scope and shadowing rules.

A parameter name may not be duplicated by another parameter in the same declaration.

Nested scopes may shadow names only where the general scope rules permit it.

## 16. Function calls

A call uses:

```sec
FunctionName(argument, argument)
```

Example:

```sec
let sum := Add(10, 20)
```

The call expression type is the selected function's declared return type.

The number and types of supplied arguments must satisfy the selected callable signature, including any variadic parameter.

Argument conversions use ordinary conversion and overload rules.

## 17. Argument evaluation order

Function and method arguments are evaluated strictly left-to-right.

```sec
Process(CreateFirst(), CreateSecond(), CreateThird())
```

has this source-semantic order:

```text
1. CreateFirst()
2. CreateSecond()
3. CreateThird()
4. enter Process(...)
```

The compiler may optimize the machine implementation only when the observable behavior is preserved.

The same left-to-right rule applies when ordinary arguments and spread arguments are mixed.

## 18. Call transfer commit

Ownership transfer to callee-owned parameters is committed only after all arguments have been evaluated successfully and the call is ready to enter the callee.

Conceptually:

1. resolve the callable;
2. evaluate arguments left-to-right;
3. perform required conversions and prepare borrows/owned temporaries;
4. if argument evaluation fails, clean caller-owned temporaries and do not enter the callee;
5. once all arguments are ready, commit consuming ownership transfers;
6. enter the callee.

Example:

```sec
let resource := OpenResource()

try Use(
    resource,
    LoadConfiguration(),
) {
    Err(error) => {
        Handle(error)
    }
}
```

If evaluating the later fallible argument fails before call entry, ownership of `resource` has not been committed to `Use`.

Caller-owned temporaries created by earlier argument expressions are cleaned according to normal failure cleanup.

This rule does not roll back effects or ownership transfers that occurred *inside the evaluation of an argument expression itself*. For example, if the first argument expression calls another consuming function, that inner call has its own completed semantics. What is delayed is the outer call's transfer from prepared argument values into the outer callee parameters.

This rule prevents partial ownership transfer caused only by failure while evaluating later arguments for the same outer call.

## 19. Ordinary overloads

Functions may be overloaded when their overload parameter signatures differ.

```sec
fn Print(value: int) void {
    ...
}

fn Print(value: string) void {
    ...
}
```

Return type alone never distinguishes overloads.

Invalid:

```sec
fn Convert(value: int) string {
    ...
}

fn Convert(value: int) bool {
    ...
}
```

Call arguments must be sufficient to select the callable.

Named types preserve their normal distinct identity during overload resolution.

## 20. Overload ranking

Overload resolution uses the ordinary conversion ranking rules.

Exact parameter matches are preferred over matches requiring conversions.

The compiler must not introduce a conversion solely to choose an otherwise inferior overload.

If several candidates remain equally valid, the call is ambiguous.

A non-variadic candidate is preferred over an otherwise equally ranked variadic candidate.

This prevents a catch-all variadic overload from unnecessarily replacing a more specific fixed-arity overload.

Ownership mode must not be chosen through surprising overload resolution.

## 21. Generic functions and methods

Functions and methods may use the generic rules defined by `generics.md`.

```sec
fn Identity[T](value: T) T {
    return value
}
```

Constraints use `:`.

```sec
fn Save[T: Serializable](value: T) Result[void, IOError] {
    ...
}
```

Multiple constraints use `&`.

```sec
fn Save[T: Serializable & Comparable](value: T) void {
    ...
}
```

Method-level generics are supported.

```sec
impl Stack[T] {
    fn Map[U](mapper: fn(T) U) Stack[U] {
        ...
    }
}
```

Partial explicit generic prefixes and inference follow the generic rulebook.

## 22. Methods and implicit `self`

An ordinary concrete method is declared with `fn` inside an implementation.

```sec
impl Counter {
    fn Increment() void {
        self.value += 1
    }
}
```

Instance methods have implicit `self`.

The programmer does not declare:

```sec
ref self
```

or:

```sec
ref mut self
```

in the method parameter list.

Sema analyzes the method body and infers whether the receiver is:

- shared/non-mutating;
- mutable/exclusive;
- consuming.

That inferred requirement participates in interface conformance and call legality.

## 23. Static methods

A type-level method is declared with `static fn`.

```sec
impl Counter {
    static fn Zero() Counter {
        return Counter {
            value: 0
        }
    }
}
```

A `static fn` has no receiver and may not use `self`.

Static function semantics are defined further by `static.md`.

## 24. Interface method contracts

Interfaces have no method bodies from which receiver capability can be inferred.

Therefore interface method declarations explicitly state receiver capability.

```sec
interface Resource {
    fn Status() Status
    mut fn Reset() void
    -> fn Detach() Handle
    static fn Parse(value: string) Result[Resource, ParseError]
}
```

The meanings are:

```text
fn         shared/non-mutating receiver contract
mut fn     mutable/exclusive receiver contract
-> fn      consuming receiver contract
static fn  no receiver
```

Concrete implementations continue to use ordinary `fn`; Sema verifies inferred concrete receiver behavior against the interface contract.

The complete interface rules are defined by `interfaces.md`.

## 25. Function values

A named function may be used as a function value only after one concrete callable has been resolved.

The callable type preserves:

- parameter types;
- borrow modes;
- consuming parameter modes;
- variadic shape;
- return type;
- fallibility represented by the return type;
- any other callable property required by the type system.

Example:

```sec
fn Release(-> resource: Resource) void {
    ...
}
```

A compatible callable type must preserve the consuming parameter contract.

An unresolved overload set is not one runtime function value.

An unresolved generic function template is not one runtime function value.

Generic functions must be concretely specialized before becoming runtime callables.

Closure capture semantics and lambda-created callable values are defined by the lambda rulebook.

Machine-level function pointer representation and foreign callback ABI are defined by ABI/FFI rules.

## 26. Result and `try` boundary

`Result[T, E]` is an ordinary generic type with language-defined error-handling semantics.

Functions may return `Result`.

```sec
fn Load() Result[Data, IOError] {
    ...
}
```

`Ok`, `Err`, bodyless `try`, local `try` handling, propagation, error-type compatibility, and related control-flow rules are owned by the error-handling rulebook.

This function rulebook does not redefine the complete Result/error model.

A function signature must nevertheless expose its declared `Result[T, E]` return type explicitly.

## 27. Discarding call results

A standalone call returning an ordinary discardable non-`void` value may discard its temporary result.

Compiler-known must-use return types must be handled according to their owning rules.

Examples include result categories such as:

- `Result[T, E]`;
- task/thread handles where the concurrency rules require handling.

Explicit `discard` may be used only where the complete returned value is itself legally discardable.

Discarding a value must never bypass required cleanup, error handling, task/thread responsibility, or other must-use semantics.

## 28. Native Sec variadic functions

Sec supports typed native variadic functions.

A variadic parameter is written:

```sec
name: ...T
```

Example:

```sec
fn Sum(values: ...int) int {
    let mut total := 0

    for value in values {
        total += value
    }

    return total
}
```

A function may declare at most one variadic parameter.

The variadic parameter must be the final parameter.

```sec
fn Log(level: LogLevel, parts: ...string) void {
    ...
}
```

Zero variadic arguments are valid.

```sec
Log(LogLevel.Info)
```

Every argument captured by the variadic parameter must satisfy the element type.

Native Sec variadics are typed. They are not C `...` varargs.

## 29. Variadic parameter pack

Inside the function, a variadic parameter is a compiler-known ephemeral parameter pack.

It is not:

- an array;
- a `list`;
- a slice with guaranteed representation;
- a heap allocation;
- a first-class storable collection type;
- a C `va_list`.

Its lifetime is limited to the current invocation.

Its physical representation is not observable by Sec source code.

The compiler may choose an efficient representation based on the call site and target, including:

- direct call-frame values;
- registers;
- stack storage;
- one or more sequence segments;
- views over already existing compatible argument storage;
- another representation preserving the normative semantics.

The compiler must not introduce semantic heap allocation merely to create a native variadic pack.

## 30. Variadic pack operations

A variadic pack supports the sequence operations required for ordinary read-only consumption:

```sec
values.Len
values[index]
```

and iteration:

```sec
for value in values {
    ...
}
```

A variadic pack may be re-spread where the resulting element transfer is legal.

```sec
Other(values...)
```

The pack does not guarantee contiguity.

Therefore:

```sec
values.Ptr
```

is invalid.

Indexing and iteration must not implicitly move an element out of the pack.

For copyable element types, ordinary value reads may produce legal copies according to the normal copy rules.

For move-only element types, code must use non-consuming/shared access where required.

## 31. Variadic pack is structurally read-only

The pack structure and its elements are not mutable through the variadic pack.

Invalid:

```sec
fn Process(values: ...int) void {
    values[0] = 42
}
```

The purpose of a variadic parameter is variable-arity argument transport, not implicit mutable collection storage.

If mutable owned storage is required, the function must create an explicit collection under the ordinary collection/ownership rules.

## 32. Variadic pack cannot escape

A variadic pack cannot outlive the invocation.

Invalid:

```sec
fn Bad(values: ...int) ref int {
    return ref values[0]
}
```

A pack cannot be stored into persistent state as the pack itself.

It cannot be captured by an escaping closure.

References into the pack must not escape the invocation.

The compiler must reject any operation that would make the ephemeral pack or a pack-backed reference survive beyond the call lifetime.

## 33. Ownership of variadic elements

Each argument received by `...T` follows ordinary by-value parameter semantics for its element type.

For copyable `T`:

```text
each supplied argument is semantically copied into the call
```

For move-only `T`:

```text
each individually supplied argument transfers ownership into the call
```

Example:

```sec
fn ConsumeAll(values: ...Resource) void {
    ...
}

ConsumeAll(first, second)
```

After successful call entry, `first` and `second` are consumed if `Resource` is move-only.

The pack owns the received values for the duration of the call unless ownership is otherwise transferred through a separately legal operation.

Remaining owned elements are destroyed according to ordinary destruction rules when the invocation ends.

## 34. No element move-out from a variadic pack

Individual elements may not be moved out of a variadic pack.

Invalid for move-only `Resource`:

```sec
fn Bad(values: ...Resource) Resource {
    return values[0]
}
```

This prevents partial-move state inside the compiler-created pack.

Shared observation and borrowing remain permitted according to the element type and borrowing rules.

A re-spread that would require moving elements out of a move-only pack is invalid.

A re-spread of copyable elements may copy those elements according to ordinary spread/call rules.

## 35. Spread arguments into variadic parameters

Ordinary arguments and spread arguments may be mixed.

```sec
fn Write(values: ...byte) void {
    ...
}

let data: byte[] := ...

Write(
    0x01,
    data...,
    0xff,
)
```

Multiple spread sources may appear when the spread rules permit them.

All argument expressions and spread sources are evaluated left-to-right.

A spread source is evaluated exactly once.

Runtime-length spread is valid because the variadic destination explicitly accepts runtime arity.

Spread itself does not gain consuming or partial-move semantics.

Therefore a spread from a collection of move-only elements is invalid when satisfying the call would require moving those individual elements out of the collection.

This preserves the canonical non-consuming spread model.

## 36. No consuming variadic parameter syntax

Sec 0.1 does not permit:

```sec
fn Consume(-> values: ...T) void {
    ...
}
```

`...T` already applies ordinary by-value semantics to every individual argument.

For move-only `T`, individually supplied values are consumed naturally.

Adding `-> ...T` would force consuming behavior for copyable elements and would create difficult partial-move semantics for spread sources.

Therefore `->` and `...` do not combine in one parameter declaration.

This is an explicit Sec 0.1 rule, not a postponed syntax.

## 37. Variadic overload resolution

Variadic shape participates in callable matching.

A fixed-arity overload is preferred over an otherwise equally ranked variadic overload.

Example:

```sec
fn Print(value: int) void {
    ...
}

fn Print(values: ...int) void {
    ...
}
```

```sec
Print(10)
```

selects the fixed-arity overload.

```sec
Print(10, 20)
```

selects the variadic overload when otherwise valid.

Conversions for each variadic argument follow the normal conversion ranking rules.

## 38. Native variadics are not foreign varargs

A Sec native declaration:

```sec
fn Format(values: ...FormatValue) string {
    ...
}
```

defines a typed Sec variadic pack.

It does not define C-style heterogeneous varargs.

Foreign declarations such as C variadic functions have ABI-defined rules including representation, default argument promotions, calling convention, and foreign safety constraints.

Those rules belong to `abi.md` and `ffi.md`.

Native `...T` semantics must not be inferred from C `va_list`, and C varargs must not be inferred from native Sec `...T`.

## 39. Recursion

Functions may call themselves directly or participate in mutually recursive call graphs where ordinary declaration and analysis rules permit.

Declaration order does not make a valid recursive call invalid.

Recursion does not bypass:

- ownership analysis;
- borrow analysis;
- effect analysis;
- stack/resource analysis;
- termination-related analyses where enabled.

## 40. ABI boundary

Ordinary Sec function semantics are source-language semantics.

Physical calling convention is separate.

A by-value parameter may physically be passed in registers, on the stack, indirectly, or in split form while preserving the same Sec ownership semantics.

A consuming `->` parameter does not require a special machine calling convention merely because it consumes the caller value.

A native variadic pack does not promise one stable physical layout.

ABI-visible/exported functions, calling-convention selection, foreign-compatible signatures, symbol naming, aggregate passing, return classification, and foreign varargs are defined by `rules/platform/abi.md` and the FFI rulebook.

Generic functions must be concretely monomorphized before they can have a concrete ABI.

## 41. Parser requirements

The parser must support:

- ordinary named functions with bodies;
- explicit return types;
- comma-separated parameters;
- trailing parameter commas;
- owned by-value parameters;
- `ref T` parameters;
- `ref mut T` parameters;
- consuming `-> name: T` parameters;
- native variadic `name: ...T` final parameters;
- generic function/method parameter lists;
- calls;
- spread arguments;
- return statements.

The parser must reject or preserve for clear Sema diagnostics:

- missing return type;
- ordinary bodyless function declaration;
- duplicate parameter names;
- `->` combined with `ref`;
- `->` combined with `ref mut`;
- `->` combined with `...`;
- more than one variadic parameter;
- a non-final variadic parameter.

## 42. Sema requirements

Sema must:

- register function signatures before bodies;
- resolve parameter and return types;
- create function scope;
- define parameters in function scope;
- treat owned by-value parameter bindings as mutable;
- preserve borrow semantics for `ref` and `ref mut`;
- classify ordinary by-value copy/move transfer;
- enforce forced consuming transfer for `->`;
- reject ownership-only duplicate overloads;
- type-check return statements;
- enforce one explicit return type;
- enforce single-value return;
- resolve overloads without using return type as a discriminator;
- prefer exact matches over conversions;
- prefer fixed arity over otherwise equal variadic matches;
- infer and instantiate generic calls according to `generics.md`;
- evaluate call argument effects in left-to-right semantic order;
- model call-transfer commit after successful argument evaluation;
- create and type native variadic packs;
- prevent variadic pack mutation, escape, pointer exposure, and element move-out;
- enforce spread legality for variadic calls;
- preserve must-use result rules;
- infer concrete method receiver capability from method bodies.

## 43. Semantic IR requirements

Semantic IR must preserve enough information to represent:

- concrete function identity;
- parameter order;
- parameter value type;
- by-value versus borrow mode;
- forced consuming parameter mode;
- variadic element type and variadic position;
- return type;
- call argument source order;
- ownership-transfer commit;
- ordinary call versus foreign call;
- static versus instance method;
- inferred concrete receiver capability;
- must-use result category where applicable.

A call lowering must not erase ownership semantics merely because the target ABI physically copies bits.

Native variadic packs must remain distinguishable from ordinary arrays/slices and foreign varargs until lowering has selected a legal representation.

## 44. Required diagnostics

The compiler must diagnose at least:

```text
function Calculate must declare an explicit return type
```

```text
ordinary function Calculate requires a body
```

```text
duplicate parameter value
```

```text
function Calculate must return int
```

```text
function Calculate must return int, got bool
```

```text
return value is not permitted from void function Notify
```

```text
multiple return values are not supported
```

```text
no matching overload for Process
```

```text
ambiguous call to Process
```

```text
function overloads cannot differ only by consuming parameter mode
```

```text
consuming parameter cannot use ref type
```

```text
consuming parameter cannot use ref mut type
```

```text
consuming variadic parameters are not supported
```

```text
variadic parameter must be last
```

```text
function may declare only one variadic parameter
```

```text
cannot mutate variadic parameter pack
```

```text
variadic parameter pack cannot escape this call
```

```text
cannot move element out of variadic parameter pack
```

```text
spread would require consuming elements from a non-consuming spread source
```

Diagnostics should identify the function, parameter, argument, or source expression responsible for the violation.

## 45. Best practice

- Keep function signatures explicit and self-describing.
- Use ordinary by-value parameters for ordinary ownership semantics.
- Use `->` only when consuming a copyable argument is semantically important or when the API should make ownership consumption unconditional across generic instantiations.
- Use `ref` for shared access and `ref mut` for exclusive mutable access without ownership transfer.
- Do not create ownership-only overload pairs.
- Prefer a named struct or another explicit type when returning several related values.
- Use native `...T` when variable arity is genuinely part of the API, not merely to avoid defining a collection parameter.
- Keep variadic element types specific; native Sec variadics are typed.
- Do not depend on variadic pack layout, contiguity, or pointer identity.
- Keep foreign varargs behind explicit FFI wrappers whenever practical.
- Let compiler analysis carry receiver and ordinary move/copy complexity instead of adding boilerplate to concrete method syntax.

## 46. Cross-rulebook ownership

This rulebook owns ordinary function declarations, ordinary parameters, explicit consuming parameters, calls, return shape, overload basics, evaluation order, call transfer commit, and native typed variadics.

Related rules are owned elsewhere:

- `init`, `free`, primary/extended implementations, implicit `self`, and `new`: `rules/declarations/impl.md`;
- interface receiver contracts: `rules/declarations/interfaces.md`;
- generic functions, method-level generics, constraints, inference, and monomorphization: `rules/declarations/generics.md`;
- static methods: `rules/declarations/static.md`;
- copy/move classification and destruction responsibility: memory copy/move rules;
- borrowing and references: borrowing rules;
- `Result`, `Ok`, `Err`, `try`, and propagation: error-handling rules;
- spread source semantics: spread rulebook;
- lambda capture and closure representation: lambda rulebook;
- effects and analyses: compiler analysis rulebooks;
- concrete physical calling conventions and exported ABI: `rules/platform/abi.md`;
- foreign declarations, callbacks, and foreign varargs: FFI rulebook.
