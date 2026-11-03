# Ideas for Sec version 0.2

**Snapshot date:** 2026-10-29  
**Snapshot revision:** Final consolidated working file, 2026-10-29  
**Status:** Non-normative brainstorming and comparative design research  
**Repository baseline:** `YoePro/sec` `main` at `3e888b5b3d6a3f5a96c4d1c85c2ba6bd47351ddf` (2026-10-28)  
**Decision authority:** Sec owner; inclusion is never an implementation decision

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

## 1.5 Ergonomics and intuition as first-class goals

A powerful type system, efficient lowering, and precise semantics are
insufficient if the language is unpleasant to write or hard to predict.
Treat **ergonomics** and **intuition** as explicit quality dimensions alongside
semantics, safety, and performance; see § 31 for evaluation criteria.

Ergonomics concerns unnecessary effort, ceremony, consistency, discoverability,
and the quality of diagnostics. Intuition concerns the programmer's ability to
form a reliable mental model and predict what expressions mean. An annotation
that communicates important intent is not bureaucracy, even if it makes the
source longer. A shorter spelling that hides ownership, effects, or meaning
may be worse.

## 1.6 Design from representative source programs

Evaluate proposals with realistic Sec examples before choosing syntax or
implementation strategy. Include ordinary use, generics, ownership,
error/failure paths, effects, and interaction with existing syntax. Prefer a
small number of coherent mechanisms over isolated clever shortcuts.

The separate bug/performance review project continues to own concrete defect
audit findings; this ideas file is not a replacement for that work.


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
- numeric type candidates: `float16`, `bfloat16`, `rational`, and `complex` / `Complex[T]`
- possible advanced numeric types: `BigInt`, `BigUint`, `BigFloat`, and `float128`
- public language semantics for compiler-internal `never`
- quaternion algebra (`Quaternion[T]`) and optional rotation-specific abstractions (`UnitQuaternion[T]` / `Rotation3[T]`)
- dual numbers (`Dual[T]`) and automatic differentiation through generic mathematics
- `Polynomial[T]` as a prospective mathematical-library type, not a missing primitive
- explicit operator annotations on ordinary static overloads, with module ownership and coherence still open
- ergonomics and intuition as first-class design evaluation criteria
- effect composition and verifiable function contracts, building on existing analyses
- typed relative references and verifiable optimization obligations as research questions
- numerical fusion and SIMD as MLIR/compiler/tooling opportunities rather than automatic new source-language features

These remain ideas to investigate, not commitments.

---

# 28. Numeric type candidates and public `never` semantics

> **Status: candidates for investigation only — not approved language features.**
>
> Inclusion here does not decide inclusion in Sec 0.2, source syntax, whether a
> type is compiler-known or library-defined, its runtime representation, its
> conversion semantics, or its implementation priority.
>
> Heterogeneous variadics are already part of Sec 0.1 and must **not** remain
> on the Sec 0.2 feature backlog as an unimplemented language facility.

## 28.1 Primary candidates for investigation

| Candidate | Motivation | Open design questions |
|---|---|---|
| `float16` | Standard 16-bit binary floating point for compact numeric data, SIMD, and device interchange. | Target support, hardware versus software lowering, arithmetic promotion, literal typing, and conversions. |
| `bfloat16` | A distinct 16-bit floating-point format useful for machine learning and numerical pipelines. | Whether this is a compiler-known scalar, storage/interchange-only on some targets, and how arithmetic is performed and rounded. |
| `rational` | Exact fractional arithmetic, including unit scale factors such as `1 / 60`, without decimal approximation. | Runtime numerator/denominator widths versus arbitrary precision, normalization, allocation, overflow, zero denominator, contextual division, and conversions. |
| `complex` / `Complex[T]` | Complex arithmetic for signal processing, mathematics, engineering, and FFT-style algorithms. | Primitive family versus generic core/library type; supported component types; layout, units, conversion, and operator semantics. |

These four are **high-interest candidates**, not locked choices.

### `rational` and unit `Scale`

The existing 0.1 unit system already uses exact-looking expressions such as
`Scale: 1 / 60`. The current question originated from typing that property,
which uses `decimal` in the current model. Investigate whether unit metadata
should use an exact rational semantic representation, independently of whether
`rational` becomes a user-storable runtime type.

Possible user-facing behavior, **illustrative only**:

```sec
let value: rational := 1 / 3
value.ToString()  // "1/3"
```

A normalized integer rational may format as `"5"` rather than `"5/1"`.

Conversions to `decimal`, `decimal128`, `float32`, `float64`, and any new float
formats must respect Sec's existing no-hidden-precision-loss rules. The user
has suggested investigating properties such as `AsDecimal` and width-specific
`AsFloat` variants. Whether these are appropriate, and how they express
rounding/fallibility, remains open; Sec's existing typed conversion mechanism
must be considered first. Do not silently introduce a second conversion model.

**Do not change `decimal` to store fractions as part of this proposal.** That
alternative was considered but is not currently preferred. Existing
`decimal`/`decimal128` semantics should remain exact base-10 decimal semantics.

### `complex`

A type may be proposed under the name `complex`, or by an explicitly typed
generic form such as `Complex[T]`. Neither spelling nor the status as a
language primitive is decided. Natural arithmetic requires a separate review
of operator support; operator overloading is a future discussion and is **not
implicitly adopted** by adding `complex` to this list.

## 28.2 Additional possible numeric candidates

| Candidate | Motivation | Open design questions |
|---|---|---|
| `BigInt` | Arbitrary-precision signed integer arithmetic. | Core versus stdlib; allocation, limits, ownership, serialization, and conversion. |
| `BigUint` | Arbitrary-precision nonnegative integer arithmetic. | Same issues as `BigInt`, plus whether a separate public type is worthwhile. |
| `BigFloat` | Floating-point arithmetic with greater or configurable precision. | Precision context, rounding policy, exponent limits, error semantics, and memory use. |
| `float128` | Wider binary floating-point format and a more symmetrical scalar family. | Actual usefulness beyond symmetry, target portability, ABI, library routines, software fallback, and conversion semantics. |

Their presence here does not decide whether they become built-ins, trusted core
types, or ordinary standard-library types. In particular, support for large
numbers must not quietly make ordinary numeric code allocate.

## 28.3 Exposing `never`

`never` is **not** a proposed new compiler-internal concept. The compiler
already has `NeverType` in semantic analysis, `TypeNever` in Semantic IR, and
`!sec.never` in the MLIR dialect.

Investigate giving `never` a fully specified public source-language role,
rather than inventing a new underlying semantic type.

The essential meanings are different:

```text
None     a real Option value denoting absence
void     normal completion without a returned value
never    no possible normal result / uninhabited result type
```

Potential use cases include terminating functions and expressions, branch
result typing, exhaustive control flow, and diagnostics. Investigate whether
`never` may appear explicitly in source declarations, where it may appear in
type positions, and how it interacts with `panic`, `unreachable`, `return`,
fallibility, and generic inference.

**No public spelling or permission is approved solely by this entry.**

## 28.4 Ranges and `Interval[T]` are a different question

Sec already supports contextual ranges, membership, iteration, slicing, and
range contracts. In Sec 0.1 a range is not a general first-class runtime value.

That is distinct from a numerical **interval-arithmetic type**, whose values
represent sets/bounds of possible numbers and whose operators propagate those
bounds with mathematically sound rounding.

Do **not** add `Interval[T]` as another missing primitive numeric type based on
this inventory alone. If genuine interval arithmetic is needed, investigate it
as a separate library/domain feature. First-class storable ranges are also a
separate language-design question.

## 28.5 Questions to resolve before accepting any candidate

For each candidate, investigate:

1. Is compiler knowledge necessary, or can a core/stdlib type provide the same semantics?
2. What are the type identity, generic, unit, and numeric-literal rules?
3. Which operations are exact, rounded, checked, or otherwise fallible?
4. What are the representation, ABI, target and FFI requirements?
5. Can ordinary operations allocate or grow storage, and how is that made visible?
6. How does the candidate participate in formatting, `ToString()`, and conversions?
7. What operator behavior is needed, without prejudging the separate operator-overloading design?
8. Which semantics must exist in compile-time evaluation, and which at runtime?
9. Can the candidate be implemented and tested consistently across supported targets?

A candidate remains **unresolved** until Sec's owner explicitly decides its
language/library status and semantics. No implementation task should treat a
candidate entry as an implicit approval.
---

# 29. Mathematical library, numeric abstractions, and algebraic types

> **Status: exploration only.** This section extends § 28. Listing a type here
> does not approve it as a language primitive, a core type, a standard-library
> type, or a Sec 0.2 deliverable.

## 29.1 Do not confuse existing shaped types with proposed library types

Sec already has compiler/core-defined `vector`, `matrix`, and `tensor` types,
including their defined shaped arithmetic and relevant views. They are **not**
missing language features merely because another language offers similar
abstractions through its standard library.

In particular, Sec has the contextual infix operator `x` for canonical
algebraic multiplication such as matrix-by-matrix and matrix-by-vector. The
ordinary `*` retains its scalar/elementwise meaning under the shaped-type
rules. Any future user-defined operator mechanism must preserve this
separation and must not silently reinterpret `*` as matrix multiplication.

`Polynomial[T]` is not currently an established compiler-known primitive. It
appears among prospective mathematical standard-library types. References to
it should be treated as a library candidate, not as evidence that a new
primitive is required.

## 29.2 Complex numbers

Continue investigating `complex` versus `Complex[T]` as described in § 28.
A generic type in core or `stdlib/math` is a serious alternative to a
compiler-known primitive; its arithmetic should be natural and should lower
efficiently when target capabilities and representation permit.

Potential interactions include numeric promotion, explicit conversion,
component constraints, units, formatting, and later operator annotations.
Compiler knowledge should be justified by a semantic or optimization need,
not merely by the presence of familiar mathematical notation.

## 29.3 Quaternions and rotation semantics

Investigate `Quaternion[T]` as a **general algebraic type** with four scalar
components. Quaternion multiplication is generally noncommutative. The
compiler must not assume a user-defined `*` is commutative, associative,
or otherwise algebraically optimizable without a separately established
contract and valid numerical assumptions.

Distinguish the algebraic type from a rotation-specific type:

- `Quaternion[T]`: a general quaternion; no unit-length invariant.
- `UnitQuaternion[T]`: a possible constrained representation of a rotation.
- `Rotation3[T]`: a possible semantic rotation abstraction that need not
  expose its physical representation.

The last two are **alternative investigation directions**, not a decision to
provide either or both. For unit quaternions, account for floating-point
drift, normalization policies, invalid construction, and the fact that
`q` and `-q` represent the same orientation even though they are distinct
quaternions as algebraic values. Equality of representations and equality of
rotations may require different operations.

Prefer a standard-library mathematical type unless a concrete compiler or
core requirement is demonstrated.

## 29.4 Dual numbers and automatic differentiation

Add `Dual[T]` as a mathematical-library candidate. Dual numbers carry a value
and derivative information and are useful for forward-mode automatic
differentiation. Their central attraction is that existing generic numerical
algorithms may become differentiable by substituting an appropriate numeric
carrier, **without** copying algorithm implementations or inventing a special
syntax for every operation.

Investigate:

- differentiation rules for arithmetic and mathematical functions;
- treatment of transcendental functions and formatting;
- generic numeric capabilities, including integration with proposed
  operator annotations;
- supported component domains and precision;
- whether representations are efficient under ordinary monomorphization,
  Sema, Semantic IR, and MLIR lowering;
- the distinction between forward-mode dual numbers and more general
  automatic-differentiation systems.

This is not a decision to introduce a `dual` primitive or compiler-known
automatic-differentiation semantics. No universal zero-cost guarantee is
assumed; efficiency must be demonstrated for concrete implementations.

## 29.5 Polynomials and the breadth of `stdlib/math`

Consider `Polynomial[T]` together with ordinary numerical algorithms,
linear algebra, signal processing, and automatic differentiation. Where
first-class shaped types already provide a representation or an operator,
standard-library work should build on those foundations rather than create
parallel, incompatible versions.

A complete mathematical API is not achieved merely by adding a list of type
names. Evaluate construction, conversions, numerical stability, error
contracts, precision, ownership, discoverability, documentation, and the
ability to use algorithms generically with different scalar types.

---

# 30. User-defined operators through annotations on ordinary functions

> **Status: concrete Sec 0.2 proposal retained for evaluation; not decided.**
> No syntax, allowed operator set, ownership rule, or implementation plan in
> this section is normative.

## 30.1 Problem and proposed direction

Mathematical library types should be able to express natural operations
without requiring every author to write `a.Add(b)` or `Type.Multiply(a, b)`.
At the same time, Sec should not introduce arbitrary implicit operator
behavior or a second independent function declaration system.

A promising direction is to attach an **explicit operator annotation to an
ordinary static function overload**:

```sec
// Illustrative proposal, not valid Sec 0.1 syntax.
impl Complex[T] {
    @operator("+")
    static fn Add(a: Complex[T], b: Complex[T]) Complex[T] {
        // Ordinary implementation body.
    }
}
```

Under the proposal, an expression `a + b` may resolve to the annotated
function, while the function remains directly callable through its normal
name. The name `Add` is conventional, not the source of operator semantics.
The annotation adds meaningful programmer intent, so it is **not** semantic
bureaucracy merely because it adds characters.

Do **not** interpret this as an approved spelling for `@operator`, or as an
approval to introduce an `Add` interface. An interface-based mechanism was
considered but is not the preferred current exploration path.

## 30.2 Multiple overloads and all-argument matching

Sec 0.1 already supports ordinary static overload resolution using **all
supplied input argument types**. It does not select solely by a method's
receiver or by the first argument. The expected return type does not
select an overload.

Operator annotations should investigate reusing this existing mechanism,
including multiple signatures mapped to the same operator:

```text
+ (Complex[T], Complex[T]) -> Complex[T]
+ (Complex[T], T)          -> Complex[T]
+ (T, Complex[T])          -> Complex[T]
```

The signatures above are **conceptual**, not declarations with approved
ownership or conversion rules.

The example `int8 + int16 -> int32` is a useful case for examining mixed
numeric signatures and result types. It is **not** permission for a user
module to redefine the semantics of built-in integer types, or a decision
that Sec's normal built-in addition must use that result type.

## 30.3 Built-in types and module ownership

One boundary is already clear in the discussion: **ordinary user code must
not add or replace operators for the built-in types**. Built-in operator
semantics remain owned by the language/compiler and authorized core surface.

For user-defined types, investigate who may declare an operator when:

- the user-owned nominal type is the left operand;
- the user-owned nominal type is the right operand;
- both operands are different user-owned nominal types;
- the two operand types belong to different modules;
- more than one candidate is available or two imports expose competing
  declarations.

Do not assume that the left operand always owns the operator. Requiring
left-operand ownership would make natural mixed expressions unnecessarily
asymmetric. Conversely, unrestricted declarations for arbitrary type pairs
could create coherence and ambiguity problems. Resolve this explicitly
before any adoption.

## 30.4 Keep Sec's ordinary semantic guarantees

An annotated operator must not bypass existing rules:

- Both operands and their contextual typing participate in the normal,
  statically determined overload-resolution process.
- Return type alone never distinguishes overloads.
- No silent lossy conversion, boxing, allocation, or ownership repair may
  be introduced merely to find a matching operator.
- Parameter transfer/borrow contracts, mutation, allocation, failure,
  effects, and evaluation order remain observable according to Sec rules.
- Operator precedence and associativity remain language-owned rather than
  becoming customizable on each overloaded function.
- A user-defined operation provides no automatic proof of mathematical
  properties such as commutativity or associativity.
- Ambiguous resolution must be diagnosed, not silently decided by import
  order, optimization mode, or target.

Ordinary operator evaluation and function-call contracts must agree. In
particular, investigate which operand ownership modes should be legal for
operator syntax; the mere presence of `@operator` must not silently move a
value that an ordinary reader expects to retain.

## 30.5 Operator families to investigate

Natural candidates include `+`, binary `-`, unary `-`, `*`, `/`, and
appropriate comparison operators. The meaning of `%`, bitwise operators,
and an explicitly annotated `x` requires separate examination.

The same symbol may have different arities: binary subtraction and unary
negation must be distinguishable without relying on function spelling.

`x` already has a canonical shaped-algebra meaning and must stay distinct
from `*`. Any extension beyond currently supported shaped operands must be
explicit and must not break ordinary identifier use of `x`.

Do not treat `+=`, `-=`, `*=`, and similar compound assignments as mere
textual rewrites. Sec's existing single-evaluation, Place, writeback,
contract, and failure rules must remain intact. Investigate whether a
specialized in-place operation is useful or allowed and how it relates to
the binary operator without introducing surprising mutation or ownership
changes.

Do not make assignment, borrow syntax, `try`, `await`, `ref`, `is`, or
short-circuit control-flow operators freely user-overloadable merely for
symmetry. The complete operator whitelist remains open.

## 30.6 Questions for later evaluation

1. What explicit annotation spelling is the most readable and least
   surprising? Is `@operator("+")` the right shape?
2. What combinations of operand ownership and result shape are legal?
3. How are module ownership and cross-module coherence enforced?
4. What generic constraints allow an algorithm to require `+` without
   forcing a specific nominal type or an unwanted interface hierarchy?
5. How do static literal shaping and overload ranking interact?
6. Can operator calls be inlined and specialized without a mandatory
   dispatch object or other hidden overhead?
7. What effects are permissible, and how are expensive operations explained
   through tooling and documentation?

Retain this as one proposal. Do not split it into implementation work or
consider it adopted until the Sec owner evaluates these questions.

---

# 31. Ergonomics and intuition are primary design requirements

> **Status: design evaluation criteria, not an independent new language
> feature.** These criteria apply to both present and future Sec design.

Ergonomics and intuition must not be treated as incidental polish that can
wait until after the language is feature-complete. Sec is still far from
1.0; its evaluation should pay close attention to the experience of writing,
reading, debugging, and learning real programs.

## 31.1 Ergonomics

Ergonomics asks how much unnecessary work the language requires to express
legitimate programmer intent.

Investigate:

- repetitive source ceremony that adds no important information;
- unnecessary annotations or moves where the compiler already has a proof;
- routine operations that require disproportionate code;
- discovery of the correct API, idiom, or conversion;
- interoperability among generics, ownership, collections, errors, and
  concurrency;
- quality of compiler diagnostics, LSP guidance, and documentation.

**Less syntax is not inherently better.** An explicit annotation is useful
when it communicates intent, constraints, permissions, or a genuine semantic
choice. Removing such information can worsen ergonomics by making code
harder to understand and maintain.

## 31.2 Intuition

Intuition asks whether a programmer with a reasonable understanding of
Sec's model can predict what code means and how related features behave.

Examine:

- whether familiar expressions have surprising side effects or cost;
- whether fields, properties, functions, and operators convey their
  differences clearly;
- whether similar concepts behave consistently across API families;
- whether hidden conversions, ownership effects, error propagation, or
  synchronization contradict reasonable expectations;
- whether exceptions to the normal rules are rare and justified;
- whether a learner can form a small mental model that generalizes.

The goal is not to copy C, C++, Rust, Go, or any other language's historical
habits. A novel model may be intuitive **after its principles are learned**.
What matters is that the model supports reliable prediction, not merely
initial familiarity.

## 31.3 Five independent dimensions

For new ideas and revisions, consider at least:

| Dimension | Evaluation question |
|---|---|
| Semantics | Is behavior defined, consistent, and compositional? |
| Safety | Can relevant misuse be prevented or clearly diagnosed? |
| Performance | Can the program execute efficiently with predictable, explainable costs? |
| Ergonomics | Does the programmer avoid unnecessary work and ceremony? |
| Intuition | Can the programmer reasonably predict behavior and find the right idiom? |

A proposal that improves one dimension may harm another. Those trade-offs
must be made explicit; no single dimension automatically wins every case.

## 31.4 Evaluate source examples before implementation plans

Before committing to a new syntax or behavioral rule, write small,
representative programs covering normal use, boundary cases, generic use,
error paths, and interactions with existing language facilities.

Ask whether the most common path is obvious, whether an annotation adds
meaning, whether the result reads naturally, and whether the same mental
model survives edge cases. Compare against how a user would solve the same
problem with **existing** Sec constructs.

This is language-design evaluation, not a replacement for the project's
separate bug, implementation, or performance-audit workflows.

---

# 32. Effect composition and verifiable contracts

> **Status: investigation that refines §§ 19 and 24.** Do not infer that an
> effect-handler language, exception system, or particular annotation syntax
> has been approved.

Sec already has substantial ownership, effect-analysis, function-contract,
and runtime-check foundations. A worthwhile 0.2 investigation is how these
facts **compose** through ordinary functions, callbacks, generic algorithms,
operator annotations, and asynchronous workflows.

Questions include:

- Can the compiler infer a higher-order function's effects from the
  concrete callback and its invocation contract?
- Can a verified no-allocation/no-blocking guarantee survive generic
  specialization, inlining, and calls across modules?
- How do suspension, cancellation, panic, explicit `Result` failures, and
  cleanup interact under composition?
- Can caller-visible preconditions, postconditions, and result-dependent
  facts be verified and preserved through separate compilation?
- When may proven facts justify check elimination, and when must a required
  runtime check remain?

Koka is a useful comparison for effect composition. Ada/SPARK is a useful
comparison for verified contracts. Neither should be adopted wholesale.

A particularly important principle is that a safety-critical contract
must not lose its guarantee because a compiler setting disables the runtime
check. Proving a check unnecessary and simply switching it off are different
things. This reinforces Sec's existing static-proof-first approach rather
than replacing it with a new contracts mechanism.

---

# 33. External ideas: keep language, library, and tooling separate

> **Status: research inventory.** An outside language's implementation is an
> example to compare, not evidence that Sec is missing a feature or must
> adopt the same syntax.

## 33.1 MLIR, SIMD, and numerical optimization

Sec's MLIR investment was made in part to support optimization and target-
specific lowering. Ordinary SIMD vectorization, algebraic fusion,
hardware-instruction selection, and lowering choices normally belong in
compiler infrastructure and tooling, **not automatically in Sec source
syntax or new language primitives**.

Study C++26's data-parallel and linear-algebra facilities and Julia's
broadcast fusion as comparisons for optimization quality and library
completeness. Be clear about the division:

```text
language/core
    owns observable type and operator semantics

stdlib/math
    owns reusable mathematical algorithms and APIs

compiler / Semantic IR / MLIR / tooling
    proves and performs legal optimization and target lowering
```

Explicit masks, lane operations, gather/scatter, target capabilities,
or placement controls may still warrant a public API when they express
**programmer intent that cannot safely be inferred**. That is a separate
question from whether the backend should automatically vectorize a loop.

## 33.2 Goose: useful challenges to storage and proof models

Goose offers interesting comparison points, especially:

- stable references into certain growable arrays through its restricted
  compiler-managed data-stack model;
- compact typed relative offsets/links;
- variable-size packed data and enum representations;
- direct construction into final destinations;
- proof-oriented `checkfree` functions that reject residual bounds checks.

These do **not** imply that Sec should replace its general allocation,
ownership, or concurrency model with Goose's restrictions.

Potential investigations:

- Can typed offset-based references or region-relative storage be made
  safe within Sec's provenance, lifetime, and relocation rules?
- Can alternative compact layouts be exposed without contradicting
  ordinary copy/move, address stability, or layout contracts?
- Can tooling verify specific optimization obligations, such as an absence
  of residual bounds checks, without weakening safe runtime behavior?

Destination-based construction is already listed in § 25 and must not be
counted as a newly discovered 0.2 feature. The project's separate
performance/bug-audit process should own concrete implementation defects.

## 33.3 Other comparisons worth retaining

| Source | Useful question for Sec | Boundary / caution |
|---|---|---|
| C++26 `inplace_vector` | Are bounded dynamic collections well specified and ergonomic? | Sec already has `list[T, Capacity]`; avoid inventing a duplicate type. |
| C++26 execution model | Can async work be composed, cancelled, and observed clearly? | Sec has its own Task/channel/concurrency model; do not add a parallel runtime philosophy by default. |
| C++26 hazard pointers / RCU | How should safe reclamation be expressed in public APIs? | Existing Sec reference, reclamation, and IR work must be the starting point. |
| C++26 saturating arithmetic | Is the public checked/wrapping/saturating surface complete? | Existing semantics mention these modes; verify coverage before proposing new primitives. |
| C++26 `function_ref` and Swift | How ergonomic are nonescaping borrowed callbacks? | Prefer Sec's existing borrow and escape models when sufficient. |
| Julia | How do numerical algorithms use multiple argument types and fused shaped operations? | Sec already resolves ordinary overloads from **all** input arguments; operator mapping and declaration ownership are the open issues. |
| C# extension operators | How are explicit operator definitions presented and conflicts handled? | Never allow arbitrary user extensions to built-in operator semantics. |
| Ada/SPARK | How can verified contracts remain useful without manual proof narration? | Avoid unchecked trust-based safety claims and optional invalidation of required guarantees. |
| Koka | How do effects compose through higher-order calls? | Do not automatically import effect handlers or exception-style `throw`. |
| Wuffs | How do range/index proofs survive multiple operations? | Do not require every Sec program to prove all bounds statically. |
| Chapel | What should future distributed shaped computation mean? | Not a current 0.2 core requirement. |

## 33.4 No duplicate feature backlog

Revalidate each researched idea against current normative rules and
implementation/governance status before considering it a gap. Several
features discussed in other languages are already present, in progress,
or intentionally different in Sec.

In particular:

- Sec already has first-class `vector`, `matrix`, and `tensor` and distinct
  shaped arithmetic with contextual `x`.
- Sec's existing ordinary overload resolution uses all input arguments.
- Bounded collections already exist as `list[T, Capacity]` and analogous
  supported collection forms.
- Heterogeneous variadics are part of Sec 0.1.
- Compiler-internal `never` already exists; § 28 asks only about a possible
  public role.
- SIMD lowering is fundamentally a compiler/MLIR responsibility unless a
  genuine programmer-visible intent needs representation.

Do not turn this research inventory into a list of mandated implementation
packages. Sec 0.2 is an exploration phase, not a deadline for importing every
interesting idea from another language.

---

# 34. Closing notes for this ideas snapshot

This file is the **consolidated, non-normative ideas snapshot** from the
current Sec 0.2 exploration. It is intentionally broad and may contain
competing alternatives. Those alternatives should be preserved rather than
silently resolved during an inventory pass.

**Keep separate:**

1. **Established 0.1 semantics:** authoritative rulebooks and accepted
   corrections govern actual language behavior.
2. **Open 0.2 ideas:** this document records directions, comparisons, and
   unanswered design questions, not approval to implement.
3. **Implementation and conformance status:** governance ledgers, packages,
   tests, and existing implementation plans own those facts.
4. **Bugs, inconsistencies, performance pitfalls, and API audits:** the
   project's separate review workflow owns concrete findings and fixes.

The Sec owner decides when an idea becomes an accepted language, core,
standard-library, or tooling requirement. No automated implementation agent
should treat a candidate in this file as approval or as a substitute for a
normative rulebook amendment.

The key future-work question is not only **"Can Sec do this?"** but also
**"Can a programmer express it naturally, understand it correctly, and
predict its consequences without unnecessary ceremony?"**

A pre-1.0 language should remain willing to refine its model when credible
examples reveal ergonomic or intuitive shortcomings. At the same time,
future changes should protect the coherent safety, ownership, and semantic
foundations already established.

