# Ideas for Sec version 0.2

> **Status: brainstorming only**
>
> This document contains ideas, observations, experiments, and possible directions for Sec version 0.2.
>
> Nothing in this document is normative and nothing becomes part of the language specification merely by being listed here.
>
> The purpose is to preserve ideas long enough for them to be evaluated properly.

---

## General direction

One of the original ideas behind Sec was to avoid **semantic bureaucracy**: syntax or annotations that force the programmer to repeat information that the compiler already knows or can reliably infer.

Sec 0.1 has nevertheless accumulated a number of such cases.

For 0.2 we should therefore continuously ask:

- Does the programmer express information the compiler already knows?
- Is a keyword adding semantic information or merely ceremony?
- Can the same intent be expressed more directly without introducing ambiguity?
- Can compiler analysis replace programmer bookkeeping?
- Can common code become shorter without hiding ownership, effects, allocation, or other important semantics?
- Can the language become more expressive without introducing multiple ways to express the same thing?

The goal is not simply fewer characters.

The goal is **less semantic bureaucracy**.

## Programmer expresses intent; compiler proves and assists

Part of the answer to the question:

> **Why another programming language?**

should be that Sec deliberately explores a different division of responsibility between the programmer and the compiler.

A possible formulation is:

> **The programmer expresses intent. The compiler acts as a companion: it proves that the intent is safe, derives what can be derived, and performs the necessary mechanical work without hiding important semantics.**

The programmer should describe what the program is intended to do.

The programmer should not be required to repeatedly prove facts that the compiler can already establish.

Likewise, the compiler should not silently invent programmer intent.

This suggests a general direction:

```text id="x3bs9n"
programmer
    expresses intent and meaningful choices

compiler
    derives mechanical consequences
    proves safety properties
    tracks ownership and lifetime facts
    propagates inferred requirements
    rejects contradictions
    explains why something is unsafe
```

The distinction is important.

Sec should not pursue convenience through hidden semantics.

Instead, Sec should pursue convenience through **stronger static understanding**.

For example:

```sec id="bb9jla"
col := GetColor()
```

The programmer expresses the intent to create a new binding.

The compiler does not need an additional `let` keyword to learn the same fact.

Likewise:

```sec id="616i93"
return Ok(result)
```

If the only semantically valid operation is to move `result` into the returned value, the compiler may be able to derive that move rather than demanding redundant ownership syntax.

Similarly:

```sec id="u8pud2"
case .GET:
```

can be sufficient when the expected enum type is already known.

And:

```sec id="3fl5bx"
values.Map(|x| x * 2)
```

may be sufficient when the surrounding API already determines the callable signature.

The same principle extends beyond syntax.

If a callable is used across a thread boundary, the compiler may derive that it must be thread-transferable.

If it can be invoked again while an earlier invocation is still active, the compiler may derive that it must be safely reusable concurrently.

If a value is tested to be a particular union or interface variant, the compiler should retain that fact within the region where the proof remains valid.

This gives Sec a broader design principle:

> **Do not ask the programmer to narrate facts that the compiler can prove.**

At the same time:

> **Do not let the compiler silently make choices that change ownership, lifetime, allocation, synchronization, effects, or other semantically important behavior.**

The boundary between those two principles is one of the central design questions for Sec 0.2.

---

# Declarations

## Let go of `let` in many cases

There is semantic bureaucracy in:

```sec id="qdrx60"
let col := GetColor()
```

The `:=` operator already tells the compiler that a new binding is being created.

This should therefore be considered:

```sec id="cfsg9i"
col := GetColor()
```

The distinction could become:

```sec id="cqqc9z"
x := 10       // declare a new immutable binding
x = 20        // assign to an existing mutable binding

mut y := 10   // declare a mutable binding
```

Explicit types could remain possible:

```sec id="3lpx3s"
x: int := 10
mut y: int := 20
```

The important rule should remain that `:=` creates a new binding.

Unlike Go, Sec should not make `:=` mean both declaration and reassignment within the same scope.

---

## Direct declaration of structs, unions, registers, etc.

Enums can already be declared directly:

```sec id="d8t1ie"
enum Enu [type] {
    A,
    B,
    C,
}
```

There is little reason for structs, unions, and registers to require an additional `type` keyword.

Instead of:

```sec id="1bnja1"
type Stru struct {
    X: int,
    Y: int,
    Z: int,
}
```

consider:

```sec id="6h7nyh"
struct Stru {
    X: int,
    Y: int,
    Z: int,
}
```

Likewise:

```sec id="ypsq19"
union Uni [T] {
    E,
    F,
    G,
}
```

and conceptually:

```sec id="rz0cc4"
register Status[32] {
    Ready: bit,
    Error: bit,
    _: bit[30],
}
```

This would make the first keyword consistently identify what is being declared:

```text id="otfvgf"
struct
union
enum
register
interface
```

The `type` keyword would remain useful for actual named types:

```sec id="nai51c"
type Percent int range 0..100
```

---

## `fn` versus `property`

Sec currently mixes very short declaration keywords such as:

```sec id="oz3x19"
fn
mut
ref
```

with full words such as:

```sec id="7lny1y"
property
interface
register
```

This is not necessarily wrong, but the syntactic style should be evaluated.

Possibilities include:

```sec id="owcs2o"
fn
property
```

or a shorter property declaration such as:

```sec id="erk70k"
prop
```

The question is broader than the exact spelling:

> Should Sec generally prefer short declaration keywords, full English words, or deliberately use both depending on frequency and clarity?

No decision yet.

---

# Arrays

## Ranges inside array literals

When creating an array, it should be considered whether ranges can expand directly into elements.

Example:

```sec id="nmvoew"
x: int[] := [100..104, 200..210]
```

Conceptually this would produce:

```sec id="s7vt8p"
[
    100,
    101,
    102,
    103,
    104,
    200,
    201,
    202,
    203,
    204,
    205,
    206,
    207,
    208,
    209,
    210,
]
```

Questions to evaluate:

- Does the range expand at compile time when constant?
- Can runtime ranges be used?
- Is this array syntax only, or should the same expansion principle work for lists and other collections?
- How does this relate to the general spread/expansion syntax?

---

# Type inference

## Inferred enum namespace

Consider:

```sec id="lg612c"
switch m {
    case http.Method.GET:  // 1. Complete bureaucracy
    case Method.GET:       // 2. Module inference
    case .GET:             // 3. Type inference
    case GET:              // 4. Implicit scope
}
```

The preferred direction is:

```sec id="sr05n2"
case .GET:
```

When the compiler already knows the expected enum type, repeating the complete namespace provides little useful information.

This idea is inspired by languages such as Odin, Zig, and Swift.

The same principle may be useful outside `switch`.

Example:

```sec id="00sikl"
method := .GET
```

when the target type is already known to be `http.Method`.

The compiler must only permit this where the type can be determined unambiguously.

---

## Flow-sensitive type narrowing

When a condition proves something about a value's type, the compiler should preserve that information inside the proven control-flow region.

Conceptually:

```sec id="d70j78"
if connection is TCPConnection {
    connection.Send(data)
}
```

Inside the branch, requiring another explicit conversion to `TCPConnection` would be semantic bureaucracy because the condition has already established the type.

This may be relevant to:

- interfaces
- unions
- generic constraints
- `Option`
- `Result`
- availability analysis
- pattern matching

The narrowing must remain valid only while the compiler can prove that the underlying fact has not changed.

---

# Conditional expressions

Sec should evaluate adding the traditional conditional expression:

```sec id="0rr9n2"
condition ? whenTrue : whenFalse
```

This is especially useful in declarations:

```sec id="zc5vp4"
port := secure ? 443 : 80
mode := debug ? .Debug : .Release
```

It provides a compact value-selection expression without turning `if` into an expression.

The conceptual distinction would be:

```text id="scde9g"
if
    control flow

?:
    conditional value selection
```

There is probably little reason to also make `if` an expression if conditional expressions already solve the common value-selection case.

Nested conditionals should not necessarily be prohibited merely because they can become complicated. Formatting and linting may help readability without artificially reducing language expressiveness.

Conditional expressions must interact correctly with ownership and availability.

Example:

```sec id="86lmlv"
selected := condition ? <-left : <-right
```

Only the selected branch is evaluated and only the selected value is moved.

---

# MATCH

Match statements should not require unnecessary binding and `where` clauses merely to inspect a nested value.

Given:

```sec id="pa802n"
enum SameSite {
    Strict,
    Lax,
    None,
    Unspecified,
}
```

The following two forms should be semantically equivalent.

### A — unnecessarily indirect

```sec id="o4vrlc"
match self.SameSite {
    Option.Some(site) where site == SameSite.Strict      => { out = out + "; SameSite=Strict" }
    Option.Some(site) where site == SameSite.Lax         => { out = out + "; SameSite=Lax" }
    Option.Some(site) where site == SameSite.None        => { out = out + "; SameSite=None" }
    Option.Some(site) where site == SameSite.Unspecified => {}
    Option.None                                          => {}
}
```

### B — direct pattern matching

```sec id="wfafj8"
match self.SameSite {
    Option.Some(SameSite.Strict)      => { out = out + "; SameSite=Strict" }
    Option.Some(SameSite.Lax)         => { out = out + "; SameSite=Lax" }
    Option.Some(SameSite.None)        => { out = out + "; SameSite=None" }
    Option.Some(SameSite.Unspecified) => {}
    Option.None                       => {}
}
```

The second form should be valid.

`where` should be used to **further qualify an already matched pattern**, not as the primary mechanism for selecting a value that can naturally be expressed directly in the pattern.

Type inference may simplify this further:

```sec id="uj2rc6"
match self.SameSite {
    .Some(.Strict)      => { out = out + "; SameSite=Strict" }
    .Some(.Lax)         => { out = out + "; SameSite=Lax" }
    .Some(.None)        => { out = out + "; SameSite=None" }
    .Some(.Unspecified) => {}
    .None               => {}
}
```

This shorter form should be evaluated separately rather than assumed.

---

# Lambdas and local value expressions

## Short context-typed lambdas

The current full lambda form is deliberately explicit:

```sec id="2e4n0l"
fn(value: int) int {
    return value * 2
}
```

That is useful when the lambda exists as an independent function value or when its complete signature needs to be visible.

It becomes unnecessarily verbose when the surrounding call already determines the expected function type.

For example:

```sec id="079jw1"
values.Map(
    fn(value: int) int {
        return value * 2
    }
)
```

could potentially have a short contextual form such as:

```sec id="ynbkda"
values.Map(|value| value * 2)
```

The exact syntax is not decided.

The important idea is:

> When the surrounding context uniquely determines a lambda's parameter and result types, the programmer should not necessarily need to repeat them.

The complete lambda syntax should remain available.

---

## Immediately evaluated value blocks

A related but different idea is needed for cases where the programmer wants the **result of local computation**, not a reusable function value.

Creating and immediately invoking a lambda would be unnecessarily bureaucratic:

```sec id="wniz43"
result := (fn() ResultType {
    ...
    return value
})()
```

We should evaluate a direct scoped value-expression form.

Conceptually:

```sec id="c1f1br"
result := value {
    temp := Calculate()

    if temp.Valid {
        return temp.Value
    }

    return fallback
}
```

The syntax above is only illustrative.

The important semantic distinction is:

```text id="bgs58e"
lambda
    creates a callable value

value block
    executes immediately and produces a value
```

Such blocks could complement conditional expressions by handling computations that are too large for `?:` but still conceptually produce one value.

---

# Callable capabilities across concurrency boundaries

A function signature describes how a callable may be invoked:

```sec id="y2c20z"
fn(Connection) void
```

It does not necessarily describe all relevant properties of the callable value itself.

A named non-capturing function and a capturing closure may have exactly the same function signature while having very different ownership, lifetime, escape, and concurrency properties.

For Sec 0.2 we should investigate whether callable values need a compiler-derived capability model.

## Callable properties

Several properties should be considered independently:

```text id="54fpia"
Callable
    may be invoked with the declared signature

Movable
    ownership of the callable may be transferred

Copyable / reusable
    the callable remains usable after a use or transfer that would otherwise consume it

Escapable
    the callable may outlive the scope in which it was created

Thread-transferable
    the callable may cross a thread or task boundary

Concurrently callable
    overlapping invocations are safe

Environment-mutating
    invocation may modify captured state
```

These properties should not automatically become source-level keywords or separate public types.

They are primarily semantic facts that the compiler should derive where possible.

## Closure environments

A capturing closure consists conceptually of:

```text id="0v360r"
closure
├── code
└── environment
    ├── capture A
    ├── capture B
    └── capture C
```

The capabilities of the callable therefore depend on the capabilities of its captures.

The compiler must consider:

- ownership of captures,
- copy versus move capture,
- reference lifetimes,
- mutable captures,
- escape requirements,
- concurrency boundaries,
- aliasing,
- concurrent access to shared state,
- and whether satisfying a requirement would require hidden allocation, copying, or synchronization.

Sec must not silently introduce cloning, ownership transfer, heap allocation, locking, or another mechanism merely to make an otherwise invalid closure usable.

## Transferability is not concurrent usability

Consider:

```sec id="kt51ki"
fn Serve(handler: fn(Connection) void) Result[void, ServerError]
```

Suppose the implementation handles each connection concurrently:

```sec id="zc3u1k"
spawn thread handler(<-connection)
```

Moving a callable to one worker requires it to be transferable.

But a server may accept another connection while the first handler is still running.

The handler may therefore also need to remain available and support overlapping invocation.

Consequently:

```text id="r2qy3g"
thread-transferable
    does not imply
concurrently callable
```

A move-only closure may be perfectly valid for one worker but unsuitable as a reusable server handler.

Likewise, a closure containing mutable state may be valid when exclusively invoked but invalid when several concurrent invocations would access the same environment.

## Infer requirements from use

The preferred Sec direction is that the programmer expresses the intended operation and the compiler derives the callable requirements.

Conceptually:

```text id="3sxz5q"
handler:
    signature = fn(Connection) void
    must cross thread boundary
    must remain available after spawn
    may have overlapping invocations
```

The source declaration may still remain:

```sec id="ubkyob"
fn Serve(handler: fn(Connection) void) Result[void, ServerError]
```

However, requirements inferred from the implementation cannot remain merely local compiler knowledge.

If they affect which callers are valid, they must become part of the compiler-visible semantic contract of `Serve`, even if they are not written explicitly in Sec source.

This is necessary for separate compilation, interfaces, generic checking, LSP information, and diagnostics.

## Relation to linearity

Callable capabilities may interact directly with optional linear semantics.

A callable may conceptually be:

```text id="q70cle"
reusable
    may be invoked repeatedly

one-shot
    must or may be consumed by one invocation

transfer-only
    may be moved to another owner but not duplicated

concurrently reusable
    overlapping invocations are permitted
```

This suggests that linearity should not be designed independently from callable analysis.

A one-shot closure is particularly interesting because its callable signature may still be:

```sec id="c577p0"
fn(Connection) void
```

while its ownership contract permits only one consuming invocation.

## Relation to purity and effects

Callable capability analysis should also interact with effect analysis.

For example, the compiler may need to know whether a closure:

- mutates its own environment,
- mutates externally visible state,
- performs I/O,
- synchronizes,
- accesses volatile state,
- or invokes other effectful operations.

Purity, thread safety, transferability, and reusability are different properties, but they may depend on overlapping compiler analysis.

They should therefore not evolve into unrelated parallel systems.

## Diagnostics

Diagnostics should identify both:

1. the operation that introduced a callable requirement, and
2. the capture or property that prevents the callable from satisfying it.

For example:

```text id="ruocmf"
handler cannot be used concurrently by spawned threads

Serve may start another handler while an earlier invocation is still running.

capture "state" provides mutable access to the same state from both invocations
```

or:

```text id="rt9odf"
handler cannot cross the thread boundary

the closure owns "resource", whose type is not transferable to another thread
```

or:

```text id="5n5phx"
handler would be consumed by this spawn

the surrounding loop requires it to remain available for later connections
```

## Direction to investigate

Sec 0.2 should investigate a unified static model connecting:

- function values,
- closures,
- captures,
- ownership,
- copying and moving,
- borrowing,
- linearity,
- escape analysis,
- thread and task transfer,
- concurrent invocation,
- and effects.

The initial preference should be:

> Infer semantic requirements from actual use and expose source syntax only where programmer intent cannot otherwise be expressed reliably.

The exact internal capability set should emerge from these existing analyses rather than becoming an independent type system of its own.

---

# Required fields and construction

## `required`?

Languages such as V provide a `required` field attribute.

The underlying problem is relevant, but the same solution may not fit Sec.

A cleaner Sec principle may be:

> A field without an explicit default must be supplied during construction.

Example:

```sec id="grirlj"
struct User {
    Name: string
    Age: int
}
```

requires:

```sec id="3uiwym"
user := User {
    Name: "Jonas"
    Age: 55
}
```

while:

```sec id="u1ohww"
struct User {
    Name: string
    Age: int := 0
}
```

could permit:

```sec id="257c7i"
user := User {
    Name: "Jonas"
}
```

This would make fields **required by default**, avoiding another annotation.

Evaluate rather than decide yet.

---

## Restricting direct construction

Some types must preserve invariants and should perhaps not be directly constructible by arbitrary code.

Example:

```sec id="o0b9te"
struct Connection {
    ...
}
```

may need to be constructed only through validated operations.

We should evaluate whether Sec's existing visibility, constructor, and invariant mechanisms already solve this completely.

Avoid introducing an additional `noinit`-style feature unless it expresses something the existing system cannot.

---

# Function arguments and configuration

Large argument lists quickly become unreadable:

```sec id="br9fzp"
Connect(host, 443, true, 30, false)
```

Sec should evaluate better ways to express configuration-heavy APIs.

Possible directions include true named arguments:

```sec id="x3v1e3"
Connect(
    host,
    Port: 443,
    TLS: true,
    Timeout: 30<s>,
)
```

explicit option structs:

```sec id="psxkyn"
Connect(
    host,
    ConnectionOptions {
        Port: 443,
        TLS: true,
        Timeout: 30<s>,
    },
)
```

or some form of context-inferred option struct:

```sec id="i5unc7"
Connect(
    host,
    {
        Port: 443,
        TLS: true,
        Timeout: 30<s>,
    },
)
```

No preferred solution yet.

The goal is to improve readability without creating unnecessary overloads or hiding important behavior.

---

# Formatting and literals

## Numeric digit separators

Evaluate allowing `_` as a visual separator in numeric literals.

Examples:

```sec id="k2r8jh"
10_000
100_000_000
0xFFFF_FFFF
0b1111_0000
```

The separator has no semantic meaning and exists only for readability.

Questions:

- Permit it in all numeric bases?
- Permit it in decimal fractional values?
- Permit it in exponents?
- What placements are invalid?
- Should the formatter normalize grouping?

---

# Copy, move, and semantic bureaucracy

Sec must remain strict about ownership, copying, moving, and borrowing.

However, explicit syntax should not be required where the semantic result is unavoidable and unambiguous.

For example:

```sec id="5lrzvx"
return Ok(<-result)
```

is a candidate for unnecessary bureaucracy.

If `result` must necessarily leave the current ownership context through the return value, then:

```sec id="d60b05"
return Ok(result)
```

could allow the compiler to infer the move.

The same principle should be evaluated for other contexts where there is only one semantically valid ownership operation.

This does **not** mean making moves generally implicit.

The distinction should be between:

```text id="5p5yzk"
ambiguous ownership transfer
    explicit syntax remains valuable

unavoidable ownership transfer
    explicit syntax may merely repeat what the compiler already knows
```

The objective is to preserve visible ownership intent where it matters without mechanically demanding markers where no alternative interpretation exists.

---

# Linear types

Sec 0.1 ownership is primarily affine:

```text id="dhwr76"
a value may be consumed at most once
```

Sec 0.2 should explore **optional linear semantics**:

```text id="pvmv6u"
a linear value must be consumed exactly once
```

Linear types must remain opt-in. Making all Sec values linear would create substantial unnecessary burden.

Potential uses include:

- transactions
- protocol state
- one-shot capabilities
- acknowledgement tokens
- state-transition tokens
- resources requiring explicit semantic completion

Example concept:

```sec id="grhq01"
transaction := BeginTransaction()

if success {
    transaction.Commit()
} else {
    transaction.Rollback()
}
```

Every possible path would have to discharge the linear obligation.

Automatic destruction should not necessarily count as satisfying a linear semantic obligation.

A transaction being destroyed is not necessarily equivalent to the program having explicitly committed or rolled it back.

## Linear obligations and transfer

A linear obligation should be transferable together with ownership.

A function does not necessarily have to perform the final consuming operation itself if it transfers the value to another owner that inherits the obligation.

Conceptually:

```text id="g7lh89"
linear obligation
    may be fulfilled
    or transferred
    but may not disappear
```

This should integrate with:

- ownership
- move analysis
- control-flow analysis
- partial moves
- aggregates
- interfaces
- generics
- `Result`
- `spawn`
- process transfer

## Linear types versus linear use

Haskell's linear types suggest an interesting distinction:

Linear semantics may not only be a permanent property of a type.

It may also describe **how a particular API or function is required to use a value**.

This raises the possibility of distinguishing:

```text id="r8u88s"
linear type
    every owned value carries a linear obligation

linear use / linear contract
    a particular API requires exact-once consumption
```

This needs considerably more exploration.

---

# Purity

Sec 0.2 should explore pure functions, but purity must remain conceptually separate from linearity.

They answer different questions:

```text id="x06gzq"
linearity
    How must a value be consumed?

purity
    What observable effects may an operation perform?
```

The two concepts may nevertheless strengthen each other.

A function receiving uniquely owned data may be able to mutate that data internally while still presenting a semantically pure transformation:

```sec id="wkzmyd"
pure fn Normalize(->data: Data) Data {
    ...
}
```

The syntax above is illustrative only.

The compiler might determine purity through effect analysis rather than requiring the programmer to annotate every function.

An explicit `pure` marker could instead serve as an asserted contract:

> Reject this function if analysis determines that it has observable effects.

Effects requiring investigation include:

- I/O
- global mutation
- mutation visible through aliases
- volatile/register access
- synchronization
- FFI calls
- resource destruction with observable effects
- allocation
- panic
- task/process interaction

No decision yet.

---

# Explicitly rejected or already decided ideas

## Mutable argument marker at call sites

Some languages require the caller to mark an argument as mutable when passing it to a function that may modify it.

Sec has already decided **not** to require this.

Immutable function arguments and repeated mutability annotations would create work for the programmer, compiler, and generated program without sufficient benefit.

Do not reopen this merely because another language uses it.

---

# Ideas currently worth carrying forward

The following ideas should remain visible while Sec 0.2 develops:

- make **programmer expresses intent; compiler proves and assists** a central Sec design principle
- reduce or remove unnecessary `let`
- direct `struct`, `union`, and `register` declarations
- review keyword consistency such as `fn` versus `property`
- ranges inside array literals
- contextual enum inference such as `.GET`
- stronger direct patterns in `match`
- conditional expressions using `?:`
- flow-sensitive type narrowing
- short context-typed lambdas
- immediately evaluated value blocks
- compiler-derived callable capabilities
- callable transfer, escape, reuse, and concurrency analysis
- required-by-default struct fields
- readable configuration/named arguments
- numeric digit separators
- elimination of unnecessary move markers in semantically unavoidable moves
- optional linear semantics
- linear obligations and linear API contracts
- purity and compiler effect analysis

These are ideas to investigate, not commitments.

Dokument: 58341