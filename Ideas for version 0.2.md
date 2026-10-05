# Ideas for Sec version 0.2

> **Status: brainstorming only**
>
> This document contains ideas, observations, experiments, and possible directions for Sec version 0.2.
>
> Nothing in this document is normative and nothing becomes part of the language specification merely by being listed here.
>
> The purpose is to preserve ideas long enough for them to be evaluated properly.

---

# 1. General direction

## 1.1 Less semantic bureaucracy

One of the original ideas behind Sec was to avoid **semantic bureaucracy**: syntax or annotations that force the programmer to repeat information that the compiler already knows or can reliably infer.

Sec 0.1 has nevertheless accumulated a number of such cases.

For 0.2 we should continuously ask:

- Does the programmer express information the compiler already knows?
- Is a keyword adding semantic information or merely ceremony?
- Can the same intent be expressed more directly without introducing ambiguity?
- Can compiler analysis replace programmer bookkeeping?
- Can common code become shorter without hiding ownership, effects, allocation, synchronization, or other important semantics?
- Can the language become more expressive without introducing several unrelated ways to express the same thing?

The goal is not simply fewer characters.

The goal is **less semantic bureaucracy**.

## 1.2 Programmer expresses intent; compiler proves and assists

Part of the answer to:

> **Why another programming language?**

should be that Sec deliberately explores a different division of responsibility between the programmer and the compiler.

A possible formulation is:

> **The programmer expresses intent. The compiler acts as a companion: it proves that the intent is safe, derives what can be derived, and performs the necessary mechanical work without hiding important semantics.**

The programmer should describe what the program is intended to do.

The programmer should not be required to repeatedly prove facts that the compiler can already establish.

Likewise, the compiler should not silently invent programmer intent.

Conceptually:

```text
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

Sec should not pursue convenience through hidden semantics.

Instead, Sec should pursue convenience through **stronger static understanding**.

A useful general rule is:

> **Do not ask the programmer to narrate facts that the compiler can prove.**

At the same time:

> **Do not let the compiler silently make choices that change ownership, lifetime, allocation, synchronization, effects, or other semantically important behavior.**

The boundary between those principles is one of the central design questions for Sec 0.2.

## 1.3 Annotation is not semantic bureaucracy

Annotations are welcome when they add information.

Good annotations may express:

- programmer intent,
- contracts,
- capabilities,
- hardware metadata,
- ABI requirements,
- external semantics,
- safety constraints,
- compiler-verifiable promises.

Semantic bureaucracy is different.

It is syntax that merely repeats a fact the compiler already knows.

A useful distinction is:

> **Annotations are valuable when they add intent, constraints, contracts, or metadata. Semantic bureaucracy is syntax that merely repeats an already-known fact.**

## 1.4 Physical implementation should not leak into source-level bureaucracy

Another emerging principle is:

> **Physical implementation should not leak into source-level bureaucracy when semantic independence can be proven.**

Examples include:

- avoiding explicit moves where ownership transfer is unavoidable,
- allowing the compiler to reuse dead storage,
- allowing in-place implementation of value semantics when uniqueness is proven,
- avoiding unnecessary temporary values,
- selecting equivalent ownership strategies without changing observable semantics.

A related rule should be:

> **Liveness may determine implementation strategy, but not programmer-visible meaning.**

---

# 2. Declarations

## 2.1 Let go of `let` in many cases

There is semantic bureaucracy in:

```sec
let col := GetColor()
```

The `:=` operator already tells the compiler that a new binding is being created.

Consider:

```sec
col := GetColor()
```

The distinction could become:

```sec
x := 10
x = 20

mut y := 10
```

Explicit types remain possible:

```sec
x: int := 10
mut y: int := 20
```

The important rule should remain:

```text
:=    creates a new binding
=     assigns to an existing binding
mut   makes the binding mutable
```

Unlike Go, Sec should not make `:=` mean both declaration and reassignment in the same scope.

## 2.2 Direct declarations

Instead of:

```sec
type Stru struct {
    X: int,
    Y: int,
    Z: int,
}
```

consider:

```sec
struct Stru {
    X: int,
    Y: int,
    Z: int,
}
```

Likewise:

```sec
union Uni[T] {
    E,
    F,
    G,
}
```

and:

```sec
register Status[32] {
    Ready: bit,
    Error: bit,
    _: bit[30],
}
```

The first declaration keyword should say what is being declared:

```text
struct
union
enum
register
interface
```

The `type` keyword remains useful for named/domain types:

```sec
type Percent int range 0..100
```

## 2.3 Keyword consistency

Sec mixes short keywords such as:

```text
fn
mut
ref
```

with longer forms such as:

```text
property
interface
register
```

This is not necessarily wrong, but the syntactic style should be reviewed.

For example, evaluate whether `property` should remain as-is or become shorter.

No decision yet.

---

# 3. Arrays and collection literals

## 3.1 Ranges inside array literals

Consider allowing ranges to expand directly into array elements:

```sec
x: int[] := [100..104, 200..210]
```

Conceptually:

```sec
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

Questions:

- Does a constant range expand at compile time?
- Can runtime ranges participate?
- Does the same mechanism apply to lists and other collections?
- How does this relate to spread/expansion syntax?

---

# 4. Type inference and contextual information

## 4.1 Inferred enum namespace

Instead of requiring:

```sec
case http.Method.GET:
```

or:

```sec
case Method.GET:
```

prefer contextual inference when the type is already known:

```sec
case .GET:
```

The fully qualified form remains valid:

```sec
case http.Method.GET:
```

Likewise:

```sec
Err(.InvalidUnion)
```

may be valid when the error enum type is already known, while:

```sec
Err(ErrorEnum.InvalidUnion)
```

remains fully valid and explicit.

The short form is convenience, not a replacement.

## 4.2 Flow-sensitive facts

Sec 0.2 should treat flow-sensitive facts as first-class compiler knowledge.

For example:

```sec
if connection is TCPConnection {
    connection.Send(data)
}
```

Inside the branch the compiler already knows that `connection` satisfies `TCPConnection`.

This principle may extend to:

- concrete type,
- union variant,
- enum state,
- numeric range,
- availability,
- move state,
- provenance,
- capability satisfaction,
- interface satisfaction.

Compiler-internal types and facts may be richer than source-level syntax.

A programmer should not have to restate a fact already proven by control flow.

## 4.3 Bidirectional/context-directed typing

Sec 0.2 should investigate context-directed typing more systematically.

Examples:

```sec
method: Method := .GET
```

```sec
values.Map(|x| x * 2)
```

```sec
return Ok(value)
```

```sec
fn Pick(a: ref User, b: ref User, useA: bool) ref User {
    return useA ? a : b
}
```

In each case, surrounding context already supplies useful type information.

The compiler should propagate expected types inward where doing so is deterministic and improves clarity.

---

# 5. Conditional expressions

## 5.1 Traditional conditional expression

Sec should evaluate the traditional conditional expression:

```sec
condition ? whenTrue : whenFalse
```

Examples:

```sec
port := secure ? 443 : 80
mode := debug ? .Debug : .Release
```

The distinction would remain:

```text
if
    control flow statement

?:
    conditional value selection
```

Nested conditionals should not automatically be prohibited.

## 5.2 Multi-condition value selection

Sec should also evaluate a multi-condition value expression.

Conceptually:

```sec
category := ? {
    temperature < 0  => .Freezing
    temperature < 10 => .Cold
    temperature < 20 => .Cool
    temperature < 30 => .Warm
    else              => .Hot
}
```

This is the multi-branch form of conditional value selection.

It is not `match`.

Conceptually:

```text
match
    selects by shape, variant, or pattern

? { ... }
    selects by arbitrary conditions
```

The first true condition is selected.

Value-producing arms must produce compatible types.

The exact syntax remains experimental.

Kotlin's subjectless `when` demonstrates that this semantic model is useful in practice, but Sec should retain its own syntax.

---

# 6. `in` and `not in`

Sec already uses:

```sec
if value in 100..200 {
    ...
}
```

and:

```sec
if value not in 100..200 {
    ...
}
```

Instead of adding a new `matches` keyword, Sec 0.2 should consider generalizing what the right-hand side of `in` / `not in` may describe.

For example:

```sec
if [remainder, paddingLength] in [
    {1, _},
    {0, != 0},
    {2, > 2},
    {3, > 1},
] {
    return Err(StructuredFieldError.InvalidSyntax)
}
```

The intent is:

> the left-hand value satisfies one member of the value-set or pattern-set described by the right-hand side.

Possible forms may include:

```sec
value in 1..10
value in [1, 4, 9]
value in [.Red, .Green, .Blue]
pair in [{0, _}, {_, 0}]
```

Potentially also relational patterns:

```sec
value in [< 0, > 100]
```

The exact pattern grammar remains undecided.

The important direction is:

> **Generalize `in` / `not in` instead of introducing another matching keyword for the same conceptual operation.**

---

# 7. Match and pattern semantics

## 7.1 Direct nested patterns

`where` should further qualify a pattern, not be required as the primary selector when the value can be expressed directly in the pattern.

Given:

```sec
enum SameSite {
    Strict,
    Lax,
    None,
    Unspecified,
}
```

prefer:

```sec
match self.SameSite {
    Option.Some(SameSite.Strict)      => { out = out + "; SameSite=Strict" }
    Option.Some(SameSite.Lax)         => { out = out + "; SameSite=Lax" }
    Option.Some(SameSite.None)        => { out = out + "; SameSite=None" }
    Option.Some(SameSite.Unspecified) => {}
    Option.None                       => {}
}
```

instead of binding and then using `where` merely to compare the bound value.

Contextual type inference may later permit:

```sec
match self.SameSite {
    .Some(.Strict)      => { out = out + "; SameSite=Strict" }
    .Some(.Lax)         => { out = out + "; SameSite=Lax" }
    .Some(.None)        => { out = out + "; SameSite=None" }
    .Some(.Unspecified) => {}
    .None               => {}
}
```

The shorter form should be evaluated separately.

## 7.2 Ownership-aware pattern matching

Pattern matching should not consume more ownership than required.

A useful 0.2 principle is:

> **Pattern matching should infer the weakest ownership operation sufficient for the actual use.**

Conceptually:

```text
read only       -> borrow
needs mutation  -> mutable borrow
needs ownership -> move
```

For example:

```sec
result := match value {
    .Some(v) => Inspect(v)
    .None    => false
}

Use(value)
```

should not necessarily consume `value` merely because its payload type is `@noCopy`.

However:

```sec
resource :<- match value {
    .Some(v) => v
    .None    => CreateResource()
}
```

may need to move the payload out of `value`.

Ownership of:

- the match subject,
- pattern bindings,
- and the match result

must be treated as separate questions.

---

# 8. Lambdas and local value expressions

## 8.1 Short context-typed lambdas

A full lambda remains useful:

```sec
fn(value: int) int {
    return value * 2
}
```

But when the surrounding API already determines the callable signature, consider:

```sec
values.Map(|value| value * 2)
```

The exact syntax is not decided.

The important principle is:

> When context uniquely determines lambda parameter and result types, the programmer should not need to repeat them.

## 8.2 Immediately evaluated value blocks

Sometimes the programmer wants the result of local computation, not a reusable function.

Avoid forcing an immediately invoked lambda merely to obtain a scoped expression.

Conceptually:

```sec
result := value {
    temp := Calculate()

    if temp.Valid {
        return temp.Value
    }

    return fallback
}
```

The syntax above is illustrative only.

Semantic distinction:

```text
lambda
    creates a callable value

value block
    executes immediately and produces a value
```

---

# 9. Callable capabilities

A function signature such as:

```sec
fn(Connection) void
```

describes invocation shape but not every property of the callable value.

A capturing closure may additionally have properties such as:

```text
Movable
Copyable
Escapable
ThreadTransferable
ConcurrentlyCallable
EnvironmentMutating
ExecutionDomainConstrained
```

These should primarily be compiler-derived semantic facts, not automatically source-level keywords.

## 9.1 Closure environment analysis

For a capturing closure:

```text
closure
├── code
└── environment
    ├── capture A
    ├── capture B
    └── capture C
```

the compiler must account for:

- ownership of captures,
- copy versus move capture,
- reference lifetimes,
- mutable captures,
- escape requirements,
- concurrency boundaries,
- aliasing,
- simultaneous access,
- hidden allocation risks.

No hidden cloning, ownership transfer, heap allocation, or synchronization should be introduced merely to make an invalid closure legal.

## 9.2 Transferable does not imply concurrently callable

A callable may be safe to transfer once while being unsuitable for reuse across simultaneous worker threads.

Therefore:

```text
ThreadTransferable
    does not imply
ConcurrentlyCallable
```

## 9.3 Inferred callable contracts

When an implementation uses a callable across a concurrency boundary, the compiler may infer requirements such as:

```text
handler:
    must cross thread boundary
    must remain available after spawn
    may have overlapping invocations
```

Such facts need not be written in the source declaration, but if they affect callers they must become part of compiler-visible interface metadata for:

- separate compilation,
- generic checking,
- interfaces,
- LSP,
- diagnostics.

## 9.4 Invocation cardinality

Kotlin-style contracts expose another important property:

```text
Never
AtMostOnce
ExactlyOnce
AtLeastOnce
Unknown
```

This matters for:

- closure escape,
- initialization,
- mutable captures,
- linear obligations,
- borrow duration,
- concurrency.

For example, a callback guaranteed to execute exactly once before the callee returns is very different from one that may be stored and invoked later.

Sec should investigate whether invocation cardinality belongs in callable analysis.

---

# 10. References, provenance, and escape

## 10.1 Avoid redundant `ref` on already-reference values

Given:

```sec
fn Pick(a: ref User, b: ref User, useA: bool) ref User {
    return useA ? a : b
}
```

the `ref` keyword inside the return expression would add no information if `a` and `b` are already reference values and the return type is already `ref User`.

Sec 0.2 should distinguish:

```text
creating a borrow/reference
```

from:

```text
passing an already-existing reference value
```

## 10.2 Multiple-origin provenance

Sec 0.1 intentionally keeps return-reference provenance simple.

Sec 0.2 should investigate allowing a returned reference to have several statically possible origins.

Conceptually:

```sec
fn Pick(a: ref User, b: ref User, useA: bool) ref User {
    return useA ? a : b
}
```

may produce:

```text
result provenance = { a, b }
```

The compiler should accept this if it can prove that the result remains valid for every possible origin.

No source-level named lifetime syntax should be required merely because multiple origins exist.

## 10.3 Nonescapable values

Swift's nonescapable model suggests treating escape capability as independent from copyability.

A value may be:

```text
copyable but nonescapable
noncopyable but escapable
noncopyable and nonescapable
```

This is useful for:

- references,
- spans,
- borrowed iterators,
- projected views,
- temporary capability values.

Sec should investigate an internal `CanEscape`-like capability rather than treating escape solely as a property of `ref`.

---

# 11. Places and projections

Hylo's projection model is relevant to Sec's existing distinction between Value, Place, and reference.

Some accessors may conceptually project storage rather than "return a reference".

Examples include:

```sec
buffer[index]
object.Property
```

Potential internal interpretation:

```text
expression projects a Place of type T
```

rather than:

```text
function constructs and returns ref T
```

This may simplify:

- indexing,
- properties,
- borrowing,
- mutation,
- alias reasoning,
- lifetime propagation.

Sec should not copy Hylo's memory model wholesale, but storage projection is worth investigating.

---

# 12. Internal access-mode model

Hylo's access conventions suggest a useful internal unification.

Sec Semantic IR may benefit from access modes conceptually similar to:

```text
Read
Mutate
Consume
Replace
Project
```

This could unify reasoning across:

- parameters,
- moves,
- borrowing,
- properties,
- indexing,
- closures,
- registers,
- atomic operations,
- assignment.

This is primarily an internal compiler model, not proposed source syntax.

---

# 13. Equivalent ownership implementations / method bundles

Hylo's method bundles suggest a more experimental 0.2 idea.

One logical operation may have several implementation strategies:

```text
read source and produce new value
mutate source in place
consume source and reuse storage
```

For example:

```sec
buffer.Normalize()
```

might conceptually support:

```text
borrowed implementation
in-place implementation
consuming implementation
```

Sec should only allow compiler choice between such variants when all variants have the same programmer-observable semantic result.

Permitted differences may include:

- storage reuse,
- avoided copies,
- avoided allocation,
- reuse of dead source storage.

Not permitted:

- different failure behavior,
- different observable effects,
- different logging,
- different synchronization semantics.

This needs substantial further exploration.

---

# 14. Copy, move, and semantic bureaucracy

Sec must remain strict about ownership, copying, moving, and borrowing.

However, explicit syntax should not be required where ownership transfer is unavoidable and unambiguous.

For example:

```sec
return Ok(<-result)
```

may be unnecessary bureaucracy if the only semantically valid interpretation is moving `result` into the returned value.

Consider allowing:

```sec
return Ok(result)
```

when the compiler can prove that ownership must leave.

This does **not** mean making moves generally implicit.

The distinction should remain:

```text
ambiguous ownership transfer
    explicit syntax remains valuable

unavoidable ownership transfer
    explicit syntax may merely repeat what the compiler already knows
```

The compiler should eliminate unnecessary ownership bookkeeping, but it must not silently introduce meaningful operations such as expensive copies.

---

# 15. Required fields and construction

## 15.1 Required by default

Rather than introducing a V-style `[required]` annotation, consider:

> A field without an explicit default must be supplied during construction.

Example:

```sec
struct User {
    Name: string
    Age: int
}
```

requires both fields.

If:

```sec
struct User {
    Name: string
    Age: int := 0
}
```

then `Age` may be omitted.

This keeps "required" as the default rather than another annotation.

## 15.2 Restrict direct construction

Some types must preserve invariants.

Before introducing a `noinit`-style feature, verify whether Sec's existing visibility, constructors, and contracts already solve the problem.

Avoid another keyword unless it expresses something that existing mechanisms cannot.

---

# 16. Function arguments and API labels

Large positional argument lists are difficult to read:

```sec
Connect(host, 443, true, 30, false)
```

Sec should evaluate better API labeling.

One strong direction, reinforced by both Hylo and Swift, is separating:

```text
external argument label
internal parameter name
```

Conceptually:

```sec
Connect(host, port: 443, tls: true, timeout: 30<s>)
```

The label is intentional public API information rather than merely exposing an internal variable name.

This should be evaluated against:

- named arguments,
- config structs,
- contextual config struct shorthand.

Argument labels are an example of useful annotation because they add meaning at the call site.

---

# 17. Numeric formatting

Evaluate allowing `_` as a visual digit separator:

```sec
10_000
100_000_000
0xFFFF_FFFF
0b1111_0000
```

Questions:

- all numeric bases?
- fractions?
- exponents?
- invalid placements?
- formatter normalization?

---

# 18. Linear and affine semantics

Sec 0.1 ownership is primarily affine:

```text
a value may be consumed at most once
```

Sec 0.2 should explore optional linear semantics:

```text
a linear obligation must be fulfilled exactly once
```

Potential uses:

- transactions,
- protocol states,
- acknowledgement tokens,
- one-shot capabilities,
- state transitions,
- reservations.

Automatic destruction should not necessarily satisfy a semantic linear obligation.

## 18.1 Capability decomposition

Instead of treating ownership as a single category, Sec should investigate independent capabilities such as:

```text
CanCopy
CanDrop
CanEscape
CanTransfer
CanShare
CanMutate
```

This produces useful combinations:

```text
CanCopy + CanDrop
    ordinary copyable value

!CanCopy + CanDrop
    affine / move-only value

!CanCopy + !CanDrop
    linear obligation
```

Swift's noncopyable and nonescapable work reinforces this multidimensional model.

## 18.2 Linear use versus linear type

Linearity may describe either:

```text
linear type
    every owned value carries an exact-once obligation

linear contract
    a particular API requires exact-once use
```

This distinction deserves further exploration.

---

# 19. Purity and effect analysis

Purity is separate from linearity:

```text
linearity
    how must a value be consumed?

purity
    what observable effects may an operation perform?
```

A uniquely owned value may possibly be mutated internally while preserving externally pure semantics.

Example concept:

```sec
pure fn Normalize(->data: Data) Data {
    ...
}
```

The exact syntax is undecided.

Prefer:

```text
compiler infers effects when possible

explicit annotation asserts programmer intent

compiler verifies the assertion
```

Potential effects include:

- I/O,
- global mutation,
- externally visible mutation,
- volatile/register access,
- synchronization,
- FFI,
- resource destruction,
- allocation,
- panic,
- task/process interaction,
- blocking,
- suspension,
- cancellation behavior.

---

# 20. Generic capability checking

Generic definitions should be validated against their declared capabilities whenever possible.

Example:

```sec
fn Duplicate[T](value: T) Pair[T, T] {
    return Pair[T, T] {
        first: value,
        second: value,
    }
}
```

This requires `T` to be copyable.

Rather than waiting for a concrete instantiation to fail deep inside compilation, the compiler should ideally report at definition time:

```text
Duplicate uses T more than once.

That requires T to be copyable, but the declaration does not
constrain T to a copyable type.
```

This follows the compiler-companion principle:

> derive the capability requirement from actual implementation use and explain what is missing.

---

# 21. Semantic compiler queries and call graph

The compiler already knows semantic facts that are expensive for humans and AI tools to reconstruct from source text.

Sec should expose this information through the CLI.

Conceptually:

```text
sec inspect callgraph http.Server.Serve
sec inspect callers http.Server.Serve
sec inspect callees http.Server.Serve
```

Useful options may include:

```text
--depth
--json
--dot
```

Call graph edges should be honest about precision.

For example:

```text
caller
callee
call site
direct / indirect
generic specialization
source location
```

An indirect callback must not be presented as a falsely precise direct call.

This may grow into a broader semantic query interface:

```text
sec inspect callers
sec inspect callees
sec inspect type
sec inspect implementations
sec inspect effects
sec inspect ownership
sec inspect provenance
```

Machine-readable output is especially important for AI-assisted development.

---

# 22. Region-based concurrency reasoning

Swift's region-based isolation is relevant to Sec's compiler-internal regions.

A value may be:

```text
not safely shareable
but safely transferable
```

If the compiler can prove that an entire reachable region is transferred and no old alias remains usable, crossing a concurrency boundary may be safe even when simultaneous sharing would not be.

Sec should investigate:

> **Ownership transfer across concurrency boundaries should be proven from reachability, aliasing, liveness, and isolation regions rather than relying solely on a coarse type-wide thread-safe classification.**

This integrates:

- compiler-internal regions,
- ownership,
- last-use analysis,
- callable capabilities,
- escape analysis,
- concurrency,
- linear obligations.

---

# 23. Tokio-derived concurrency ideas

Tokio exposes several runtime/library problems that Sec may be able to model more explicitly or statically.

## 23.1 Cancellation safety

Tokio's `select!` makes cancellation safety an operation-specific concern.

Some operations may safely be cancelled and restarted.

Others may:

- lose progress,
- lose queue position,
- consume partial input,
- produce partial output,
- leave state that must be handled.

Sec should investigate a compiler-visible semantic property such as:

```text
CancelSafe
```

or a richer cancellation effect model.

Conceptually:

```text
operation:
    may suspend
    may be cancelled
    cancellation preserves observable state
```

This does not necessarily imply source-level syntax.

The compiler should infer or verify cancellation properties where possible.

This is especially relevant because Sec already separates select readiness from commit.

## 23.2 Reservation / permit capabilities

Tokio bounded channels expose capacity reservations through permits.

The idea is relevant to Sec:

```sec
permit := try sender.Reserve()

value := BuildExpensiveValue()

sender.Send(<-permit, <-value)
```

The permit represents a capability:

```text
owns reservation
may commit once
or may be abandoned and rolled back
```

This is a strong example of an affine capability value and may fit Sec's future linear/affine system.

It also fits naturally with readiness/commit select semantics.

## 23.3 Task lifecycle ownership

Cancellation request and task completion are different facts.

A task group or similar lifecycle owner may need to:

- own child tasks,
- request cancellation,
- await actual completion,
- collect outcomes,
- define destruction semantics.

Conceptually:

```sec
group := TaskGroup()

group.Spawn(...)
group.Spawn(...)

group.Cancel()
outcome := await group
```

Exact API is not decided.

The important principle is:

> task aggregation is lifecycle ownership, not merely a container of handles.

## 23.4 Execution-domain capability

Tokio distinguishes normal spawned tasks from local tasks that must remain on one execution thread.

This suggests another callable/task capability:

```text
ExecutionDomainConstrained
```

A task may be valid to execute but invalid to migrate.

Sec should investigate whether the compiler can distinguish:

```text
transferable task
    scheduler may migrate

non-transferable but locally valid task
    must remain in current execution domain
```

Whether the compiler may silently choose local scheduling remains an open design question.

## 23.5 Distinct channel semantics

Tokio separates several channel concepts:

```text
mpsc
    queued messages

oneshot
    one produced value

watch
    latest state

broadcast
    event stream for several subscribers
```

These are semantically different protocols, not merely configuration flags.

Sec stdlib should later evaluate whether it also needs distinct abstractions instead of forcing every use case through a single `Channel[T]`.

## 23.6 Backpressure as protocol

Bounded channel capacity is part of concurrency semantics.

Sec should investigate explicit reservation protocols for capacity-sensitive operations, especially with:

- move-only payloads,
- expensive message construction,
- select,
- cancellation,
- transactional commit.

## 23.7 Blocking effects

Tokio requires special handling for blocking work.

Sec effect analysis may eventually include:

```text
MayBlock
MaySuspend
Cancellable
CancelSafe
```

A compiler diagnostic could detect blocking work in a task executor context and explain the problem.

Prefer diagnostics or explicit runtime facilities over silently moving work onto another thread.

## 23.8 Cooperative scheduling

A task that never reaches a suspension point may starve other work in a cooperative executor.

Sec should investigate:

- diagnostics for potentially unbounded non-suspending task loops,
- explicit yield/checkpoint operations,
- runtime scheduling guarantees.

Automatically inserting yield points may change observable timing and should therefore be treated cautiously.

## 23.9 Select fairness

Tokio demonstrates that fairness policy matters.

Sec should define fairness explicitly but should avoid making program semantics depend on subtle polling order.

The existing readiness -> choose -> commit model should remain preferable to Future-polling semantics.

---

# 24. Function contracts

Kotlin contracts suggest a useful general direction.

Functions may have semantic guarantees not expressible solely by normal parameter and return types.

Examples:

```text
callback does not escape
callback executes exactly once
return true implies some proven fact
result lifetime depends on parameter
operation preserves an invariant
```

These are good annotation candidates because they add information.

However, Sec should avoid unchecked "trust me" contracts for safety-critical semantics.

Preferred direction:

```text
compiler infers when implementation is visible

explicit annotation may state intended contract

compiler verifies the annotation
```

Compiler-visible contracts may become important for separate compilation.

---

# 25. Result-location and destination semantics

Zig's result-location model suggests that the compiler may know more than the expected type.

It may also know where a result is going.

Conceptually:

```text
ExpectedType
Destination
OwnershipExpectation
```

This may allow Sec to:

- construct values directly into their destination,
- avoid unnecessary temporaries,
- avoid redundant moves,
- improve move-only value handling.

Examples:

```sec
result := MakeSomething()
return MakeSomething()
container.Field = MakeSomething()
```

The source language need not expose physical destination management when the compiler can preserve semantics automatically.

---

# 26. Explicitly rejected or already decided directions

## 26.1 Mutable argument marker at call sites

Sec has already decided **not** to require a `mut` marker at call sites.

Do not reopen this merely because other languages use it.

## 26.2 `throw`

Sec does not adopt exception-style `throw`.

`Result`, `try`, and explicit typed error handling remain the Sec model.

## 26.3 Compile-time reflection

Compile-time reflection has been considered but is not currently a direction of interest.

Do not prioritize it merely because other languages provide it.

## 26.4 Hidden synchronization or ownership repair

Sec should not silently introduce:

- mutexes,
- cloning,
- copies,
- heap allocation,
- ownership transfer,
- thread migration,

merely to make otherwise invalid code compile.

## 26.5 Implicit dependency injection

Context-parameter systems that silently provide arbitrary service dependencies are probably not a good fit for Sec.

Compiler-visible semantic requirements are different from hidden object dependencies.

---

# 27. Ideas currently worth carrying forward

The following ideas should remain visible while Sec 0.2 develops:

- programmer expresses intent; compiler proves and assists
- annotations add information; semantic bureaucracy repeats known information
- stronger static understanding instead of hidden semantics
- physical implementation should not leak into source-level bureaucracy
- remove unnecessary `let`
- direct `struct`, `union`, `register`, and similar declarations
- review keyword consistency
- ranges inside collection literals
- contextual enum inference such as `.GET`
- preserve fully qualified enum syntax
- flow-sensitive facts
- context-directed / bidirectional typing
- direct nested patterns in `match`
- ownership-aware pattern matching
- conditional expression `?:`
- multi-condition value expression such as `? { ... }`
- generalized `in` / `not in` with value and pattern sets
- short context-typed lambdas
- immediately evaluated value blocks
- callable capability analysis
- invocation cardinality
- escape analysis and nonescapable values
- multiple-origin reference provenance
- avoid redundant `ref` where an expression is already a reference value
- Place projection
- internal access-mode analysis
- semantically equivalent ownership implementations / method bundles
- readable argument labels
- required-by-default struct fields
- numeric digit separators
- elimination of unnecessary move markers
- optional linear semantics
- independent capability decomposition (`CanCopy`, `CanDrop`, `CanEscape`, ...)
- purity and effect analysis
- definition-site generic capability checking
- semantic CLI queries and call graph
- region-based isolation for concurrency transfer
- cancellation safety as compiler-visible semantics
- reservation/permit capabilities
- task lifecycle ownership / task groups
- execution-domain constraints
- semantically distinct channel abstractions
- explicit backpressure protocols
- blocking/suspension effects
- compiler-verifiable function contracts
- result-location / destination semantics

These remain ideas to investigate, not commitments.
