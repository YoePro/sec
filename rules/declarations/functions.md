# Sec Functions

- **Status:** Normative
- **Created:** 2026-08-14
- **Last updated:** 2026-10-28 (§ 20 `Some(value)` payload in overload elimination, MD-102); 2026-10-27 (MD-117 heterogeneous variadic type packs, § 38A; § 33 example)
- **Document revision:** 2.1
- **Language version:** Sec 0.1
- **Replaces:** `rules/declarations/functions.txt`
- **Canonical path:** `rules/declarations/functions.md`
- **Implementation governance:** `governance/declarations.yaml`

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

A complete ordinary signature without a body is diagnosed as an unimplemented
function stub, with error ID `P2019` (`parser.unimplemented-function`) on the
function name. Its primary message is `Unimplemented function NAME`.
The help begins with: "The function `NAME` has been declared with a signature,
but its body is missing." It then adapts to the declared return type:

- `void`: suggest a placeholder body `{}`.
- Plain built-in integer types: suggest `{ return 0 }`.
- `string`: suggest `{ return "" }`.
- `bool`: suggest `{ return false }`.
- Other return types: request a valid value of the declared return type, without
  inventing a default for named, constrained, reference, or aggregate types.

Examples use Sec syntax without semicolons. A temporary return is suggested so
other code can compile during testing; it does not implement the intended logic.

The parser retains the signature with no body for tooling and continues with
the next declaration. This remains an error blocking compilation. An empty
placeholder body is subject to the normal return-type and control-flow rules.

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

When the argument is a reusable source place, the call site must make the
transfer visible as `Process(<-resource>)`. `Process(resource)` is invalid when
it would silently consume a non-copyable reusable source. A fresh temporary may
still be forwarded without a redundant marker.

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
let result := Transform(<-data)

Use(data)
```

The final use is invalid because `Transform` consumes `data`.

A consuming parameter is an ownership contract, not a hint about machine-level ABI passing.

The call site must show consumption of a reusable source:

```sec
Transform(<-data)
```

`Transform(data)` is invalid because the callee contract is consuming. The
requirement also applies when the concrete type is copyable. Fresh temporaries
remain marker-free. Transfer into the outer callee commits only after all its
arguments are valid and ready.

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

The explicit spelling `return <-buffer` is also valid. The marker is optional
at this terminal ownership boundary, and receiving the fresh result needs no
move marker.

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

Preparing a borrow in step 3 includes the call-lifetime borrow of a fresh owned temporary passed to a `ref T` or `ref mut T` parameter (`rules/memory/borrowing.md` § 5.4, MD-076). The temporary stays caller-owned, is cleaned at the end of the enclosing full expression, and must not be retained through the call's returned reference beyond that full expression.

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

Overloads are distinguished by their complete input-parameter signature. Return type and the type expected for a call result never select an overload.

Overload resolution considers all supplied input arguments together. Independently typed arguments eliminate inapplicable candidates before a contextual `None` or `[]` argument is typed from the single remaining parameter type; viable candidates that give such an argument incompatible types make the call ambiguous (`types.md` "Contextual None and empty-literal arguments"; MD-027, 2026-10-12):

```sec
fn Set(key: string, value: Option[int]) void {
}

fn Set(key: int, value: Option[string]) void {
}

Set("count", None)  // Valid: the first overload is selected.
Set(42, None)       // Valid: the second overload is selected.
```

Named types preserve their normal distinct identity during overload resolution.

## 20. Overload ranking

Overload resolution uses the ordinary conversion ranking rules.

Exact parameter matches are preferred over matches requiring conversions.

The compiler must not introduce a conversion solely to choose an otherwise inferior overload.

If several candidates remain equally valid, the call is ambiguous.

A non-variadic candidate is preferred over an otherwise equally ranked variadic candidate.

This prevents a catch-all variadic overload from unnecessarily replacing a more specific fixed-arity overload.

Ownership mode must not be chosen through surprising overload resolution.

A `Some(value)` argument contributes its payload type to candidate
elimination exactly like an ordinary argument (MD-102): a candidate expecting
`Option[U]` is viable only when the payload can legally initialize `U` under
the existing contextual-literal, compatibility and explicit-conversion rules.
`Store(Some(42))` selects `Option[int]` over `Option[string]`. For candidates
such as `Option[int32]` and `Option[int64]` the ordinary literal ranking
decides, or the call is ambiguous and the diagnostic names both candidate
types and suggests explicit typing; there is no `Some`-specific precedence.
Selection is deterministic and independent of declaration order, examines
payload types and static constraints only, and evaluates the payload exactly
once in the ordinary argument order (§ 17); a failed overload choice performs
no runtime evaluation. Call ownership follows the ordinary rules
(`rules/types/types.md` "Contextual and self-inferred `Some(value)`").

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
- mutable/exclusive.

That inferred requirement participates in interface conformance and call legality.

An ordinary instance method never consumes the complete `self` value. Whole
instance lifetime termination belongs to `free`. With sufficient
mutable/exclusive receiver authority, a method may still move, discard,
replace, or reinitialize an owned member when partial-ownership and borrow rules
permit it.

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
    static fn Parse(value: string) Result[Resource, ParseError]
}
```

The meanings are:

```text
fn         shared/non-mutating receiver contract
mut fn     mutable/exclusive receiver contract
static fn  no receiver
```

Ordinary interface instance methods have no whole-`self` consuming receiver
mode. `->` remains valid on ordinary parameters inside method signatures.

Concrete implementations continue to use ordinary `fn`; Sema verifies inferred concrete receiver behavior against the interface contract.

The complete interface rules are defined by `interfaces.md`.

## 25. Function values

A named function may be used as a function value only after one concrete callable has been resolved.

The callable type preserves:

- callable environment capability: shared `fn`, mutable `mut fn`, or consuming `-> fn`;
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

Callable capability describes authority over the callable value/environment,
independently of parameter ownership:

```sec
fn(int) int
mut fn() int
-> fn() Resource
```

A callable requiring less authority may satisfy a target that intentionally
permits more authority: `fn` may satisfy `fn`, `mut fn`, or `-> fn`; `mut fn`
may satisfy `mut fn` or `-> fn`; `-> fn` satisfies only `-> fn`. A conversion
must never erase a stronger source requirement.

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

ConsumeAll(<-first, <-second)
```

`<-` is required for each **named** move-only argument (`rules/memory/copy_move.md` § 8.1); a fresh temporary such as `ConsumeAll(CreateResource())` uses the ordinary fresh-value rule and needs no marker. This is a consistency correction of the example (MD-117 § 12), not a new move rule.

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

Whole-pack transfer is a **call-site** operation, `<-values...`, defined in § 38A(7) (MD-117 § 3(6)); it is not a parameter mode.

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

Those rules belong to `rules/platform/abi.md` and `rules/platform/ffi.md`.

Native `...T` semantics must not be inferred from C `va_list`, and C varargs must not be inferred from native Sec `...T`.

A C vararg marker is a bare final `...` and is legal only in an `unsafe extern
"C"` signature or `c::fn` type. Its arguments use target C default promotions,
do not accept `ref`/`ref mut` directly, and do not support Sec spread.

`c::fn(...) R` is a foreign ABI function-pointer type distinct from native
`fn`/`mut fn`/`-> fn`. `c::callback(callable)` adapts only an environment-free,
reusable callable with an exact foreign-facing signature; capturing, mutable,
and consuming callables are rejected in Sec 0.1.

## 38A. Heterogeneous variadic type packs (MD-117)

Decided 2026-10-26 (`rules/corrections/md117-heterogeneous-variadic-type-packs-correction.md`). This section supplements §§ 28–38; homogeneous `name: ...T` keeps every rule above unchanged (§ 38A(1)(4)).

**§ 38A(1) Purpose.** (1) Sec 0.1 supports heterogeneous native variadic arguments in ordinary statically typed functions and methods: a call may supply zero or more values of different concrete types without implicit `any`, type erasure, boxing, dynamic reflection or C varargs. (2) It is a general language feature equally available to user code, stdlib and core; no formatter name, `fmt.Printf` declaration or other stdlib API receives privileged call semantics. (3) Callers write the natural call; the compiler establishes the static type, ownership, lifetime and control-flow guarantees itself, without redundant caller boilerplate. (4) Homogeneous `name: ...T`, where `T` is one concrete type, remains valid and is not reinterpreted as heterogeneous. (5) The public `fmt` API, its formatting grammar, validation/error contracts, output and materialization are a separate stdlib specification (MD-118) and add no `fmt`-specific language behavior.

**§ 38A(2) Type-pack declaration.** (1) A heterogeneous type-pack parameter is written `[...Types]`; `[T]` remains one type:

```sec
fn Trace[...Types](values: ...Types) void {
    // Each argument retains its concrete type.
}
```

(2) `Types` is an ordinary programmer-chosen identifier, not a keyword or builtin type; the prefix `...` in the generic parameter list makes it an ordered sequence of zero or more types. (3) For arguments of types `string`, `int` and `bool` the inferred pack identity is the ordered sequence `(string, int, bool)`, preserving order, nominal identity, reference kind and every other relevant semantic distinction. (4) An empty pack is valid. A type pack is a compile-time generic entity, not a dynamic type, tuple object, storable container, source-level array or first-class runtime type descriptor. (5) A function or method declares at most one type pack, at the end of its generic parameter list, and at most one corresponding native variadic parameter, at the end of its value-parameter list; packs in fields, return-type packs and nested generic argument lists are not introduced. (6) Packs are inferred from ordinary concrete call arguments and statically known pack spreads; there is no runtime pack value and no pack-literal syntax.

**§ 38A(3) Parameter modes.** (1) Three forms exist:

```sec
fn ReceiveOwned[...Types](values: ...Types) void {
}

fn ReceiveShared[...Types](values: ...ref Types) void {
}

fn ReceiveMutable[...Types](values: ...ref mut Types) void {
}
```

(2) They are pointwise ordinary parameter modes: `...Types` is by-value (copy or ownership transfer as each concrete type requires), `...ref Types` a shared borrow and `...ref mut Types` an exclusive mutable borrow per expanded argument. (3) Call-site implicit borrowing, explicit `<-` for transfers from named move-only values, temporary lifetimes and mutable exclusivity apply to each expanded argument. (4) A `...ref mut Types` pack may hold several mutable references only when the ordinary non-overlap and lifetime proofs hold; a conflicting call is rejected before entering the callee. (5) The pack structure stays read-only (§ 31); mutating a referent through a granted `ref mut` does not permit mutating, resizing or rearranging the pack. (6) `-> values: ...T` and `-> values: ...Types` remain invalid (§ 36).

**§ 38A(4) Invocation, arity and inference.** (1) Calls use ordinary argument syntax, `Trace("Hello", 42, true)`. (2) For a heterogeneous pack the expanded argument count and every position's concrete type must be statically known at the call site after legal spread expansion; values may be computed at run time. (3) A homogeneous `...T` keeps its runtime-length spread (§ 35). A runtime-length source cannot infer a heterogeneous pack and is never converted into `any` or a type-erased sequence. (4) Each argument is checked against its concrete receiving mode with ordinary literal-context typing and explicit conversions; no implicit coercion among unrelated types is introduced. (5) Overload resolution is unchanged (§ 37): an otherwise equivalent fixed-arity candidate is preferred, return type never selects, there is no runtime dispatch and ambiguity is an error. (6) A generic body must be valid for each demanded concrete specialization; each operation on a pack element needs an existing operation, method, overload or satisfied constraint for that concrete element. There is no implicit structural `FormatArg` interface or `Printf` exemption.

**§ 38A(5) Pack value, indexing and iteration.** (1) A heterogeneous pack is an ephemeral compiler-known parameter pack with `values.Len`, ordered access and iteration; it promises no contiguity and has no `values.Ptr` (§ 30). (2) `values.Len` is compile-time-known for a heterogeneous specialization, including the empty pack; an index selecting a position with a concrete static type must be compile-time-known, and a runtime index never yields a fictitious universal type. (3) `for` needs no `static` keyword: for a heterogeneous pack the compiler expands iterations type-aware (`rules/control-flow/flowcontrol_for.md` § 13A); ordinary iterables keep runtime iteration, with the same observable behavior. (4) Loop bindings keep their meaning: `for value` (by value, legal only when the element is implicitly copyable), `for ref value` and `for ref mut value`. (5) `for value` never silently borrows a move-only value nor moves it out of the pack; a specialization requiring an unavailable copy is invalid; use `for ref value`. (6) In a pack of `ref T` ordinary shared reference/reborrow rules apply; in a pack of `ref mut T`, `for ref mut value` may exclusively reborrow the referent without moving the reference holder out; no nested `ref ref T` is manufactured by loop syntax. (7) `break`, `continue`, `return`, `defer`, `try`, `panic`, source-order evaluation, borrow live ranges and deterministic cleanup keep their observable semantics; static expansion and unrolling are implementation techniques, and an iteration whose body is not reached executes no side effects.

**§ 38A(6) Evaluation, transfer commit and destruction.** (1) Arguments and spread sources are evaluated exactly once in left-to-right order, which the expanded sequence preserves (§ 17). (2) The call-transfer-commit rule (§ 18) holds: ownership transfer into callee-owned parameters commits only after every argument was prepared; a failed preparation leaves no partial transfer or stale moved state, and completed temporaries are cleaned normally. (3) After call entry each owned element has one owner and one destruction responsibility; every remaining owned element is destroyed exactly once on ordinary and error exits in the existing deterministic order; borrowed referents are not destroyed by the pack. (4) Static expansion never duplicates argument evaluation, implicit moves, allocators, visible operations or destruction, and the pack needs no hidden heap allocation (§ 29).

**§ 38A(7) Spread and whole-pack consuming spread.** (1) Postfix `values...` stays non-consuming (`rules/declarations/spread.md`): it copies copyable values or passes/reborrows compatible references, and never extracts move-only elements or violates exclusivity. (2) The explicitly consuming call-site form is:

```sec
fn Forward[...Types](values: ...Types) void {
    TakeAll(<-values...)
}
```

(3) `<-values...` is legal only for a complete, currently owned native variadic parameter pack; the target must accept the whole expanded sequence with valid concrete types and ownership modes; the transfer is one logical transaction that invalidates the source pack after successful call-transfer commit. (4) It never permits `<-values[0]`, partial pack consumption, consuming spread of arrays, slices, lists, tuples or structs, or a consuming spread operator elsewhere (§ 34). (5) Transferring a complete pack keeps one owner per element without a partially moved pack state; moving one element would need per-position availability, iteration and cleanup state that the ephemeral pack contract does not define. (6) Shared-reference packs may be forwarded with ordinary `values...`; mutable-reference forwarding reborrows exclusively only when every borrow and overlap proof succeeds and never copies a `ref mut`. (7) `<-values...` applies equally to homogeneous and heterogeneous owned packs.

**§ 38A(8) Lifetime and returned-reference provenance.** (1) The pack cannot escape the invocation, be stored or be captured by an escaping closure, and a reference into storage owned by the pack cannot escape (§ 32). (2) A reference carried by a pack is distinct from a reference to the pack's own storage: a shared or mutable reference originating in caller-owned storage may be returned or forwarded when the compiler proves that the referent's lifetime, storage validity and borrow authority outlive it. (3) Such proofs use the ordinary returned-reference provenance model, including finite caller-side origin sets and per-position projections; no lifetime annotation is introduced. (4) Returning `ref mut T` preserves the exclusive grant and rejects overlapping usable mutable authority; a mutable reborrow used for a returned reference is related to the original caller-side borrow, never a new owner. (5) No common return type is synthesized; a runtime selection among different element types never becomes `any` or a universal reference. (6) Whether an access is spelled `value`, `*value` or otherwise is not decided here (MD-119).

**§ 38A(9) Specialization and Semantic IR.** (1) The ordered concrete type sequence is part of the canonical specialization identity: `(string, int)` and `(int, string)` are distinct, as are empty and nonempty packs, with nominal identities and parameter modes preserved (`rules/compiler/monomorphization.md`). (2) There is no runtime type dictionary, erased pack ABI or implicit boxing; specializations share machine code only where monomorphization proves identical observable semantics. (3) Semantic IR records the facts listed in `rules/compiler/semantic_ir.md` § 27A, verified before lowering. (4) A CompilationPlan or target ABI may affect lowering and reuse, never the pack's semantic identity; reuse fails closed when the specialization or relevant target facts differ.

**§ 38A(10) Diagnostics, tooling and FFI.** (1) Diagnostics name the failing pack position and concrete type and say whether the error concerns inference, a runtime-length heterogeneous spread, a forbidden element move, an unauthorized mutable borrow, an expired lifetime, failed returned-reference provenance, invalid whole-pack consumption or an unsupported operation in a specialization. (2) An error in a concrete expansion names the template location and the concrete instantiation or argument position, may suggest `for ref`, `for ref mut`, `<-values...` or an ordinary conversion, and never suggests a hidden `any` or boxing workaround. (3) The parser, CST, formatter, LSP, hover, signature help and diagnostics registry understand `[...Types]`, `...ref Types`, `...ref mut Types`, pack inference and `<-values...` without changing array or spread grammar. (4) No new diagnostic ID is reserved by MD-117; IDs come from the central registry. (5) Ordinary heterogeneous parameters on `main` introduce no different pack system; process-entry argument supply stays with the entry-point and platform contracts. (6) Native type packs are not C variadics (§ 38); direct foreign variadic calls keep their unsafe, ABI-compatible and promotion rules.

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
