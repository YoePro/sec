# Runtime Checks

## Status

This document is the canonical runtime-check rulebook for Sec.

The word `runtime` in this filename means:

```text
the check is evaluated while the generated program executes
```

It does not mean:

```text
a mandatory Sec runtime library
a garbage-collected runtime
a hidden exception runtime
a scheduler dependency
a required process-wide support layer
```

Sec must remain capable of producing programs without a language runtime.

Runtime checks must lower through one or more of:

```text
compile-time proof and check elimination
inline comparisons and branches
ordinary Result construction
target instructions
target traps
user-provided handlers
profile-provided handlers
small compiler-emitted helpers when explicitly permitted
```

No check may require linking a general Sec runtime.

---

# Purpose

Sec uses checked language semantics.

Operations such as:

```text
integer arithmetic
division
remainder
shifts
indexing
slicing
narrowing conversion
type-contract construction
collection capacity use
reference-generation validation
selected representation validation
assertion
unreachable
```

may require dynamic validation when static proof is insufficient.

The language must provide:

1. a checked ordinary form;
2. a panic-free fallible form;
3. explicit alternative semantics where the mathematical operation differs;
4. static proof and check elimination;
5. compiler-visible panic effects;
6. no mandatory runtime dependency.

---

# Core rule

> Every language-defined runtime failure must have a path that does not panic.

The non-panicking path may be:

```text
statically proven safe
fallible through `try`
expressed as an explicit Result-producing operation
expressed through an explicit alternative semantic such as wrapping or
saturation
```

The compiler must not force panic as the only available behavior for a
foreseeable runtime condition.

---

# Runtime-check categories

## Arithmetic

```text
signed overflow
unsigned overflow where the operation is checked
division by zero
signed minimum divided by minus one
invalid remainder divisor
invalid shift count
invalid numeric narrowing
invalid float-to-integer conversion
non-finite value where prohibited
decimal precision or scale failure where defined
```

## Bounds and shape

```text
array index outside bounds
slice index outside bounds
invalid slice range
bounded collection capacity exceeded
vector or matrix shape mismatch
tensor dimension mismatch
spread length mismatch where dynamically checked
```

## Contracts

```text
range violation
in-list violation
odd/even violation
multipleOf violation
finite violation
notEmpty violation
unique violation
require predicate failure
other named-type contract failure
```

## References and representation

```text
invalid generation
expired arena reference
invalid optional-reference unwrap
invalid enum representation from foreign or unsafe input
invalid union tag from foreign or unsafe input
invalid scalar conversion
invalid ABI decode
```

Ordinary ownership violations that are statically provable remain compile-time
errors.

## Assertions and control flow

```text
assertion failure
checked unreachable reached
explicit panic
```

These are specified primarily by `panic.md`.

---

# Three compiler outcomes

## Proven safe

The compiler proves the failure condition impossible.

```sec
type Small int range 0..100

fn Increment(value: Small) int {
    return value + 1
}
```

When the result is representable, no runtime check or panic effect remains.

## Fallible

The programmer uses `try`.

```sec
let total := try left + right
```

The check becomes explicit control flow producing a typed error.

## Panic-capable

The ordinary operation is used and failure cannot be disproven.

```sec
let total := left + right
```

The operation has a panic effect.

If failure occurs, `panic.md` defines the selected endpoint and containment.

---

# No mandatory runtime architecture

Checked addition may lower conceptually to:

```text
sum, overflow = target_checked_add(left, right)

if overflow:
    panic endpoint
else:
    continue with sum
```

A target may implement this through:

```text
overflow flag plus branch
comparison plus branch
target trap
direct call to a non-returning application handler
small local compiler-emitted helper
```

Fallible lowering may be:

```text
sum, overflow = target_checked_add(left, right)

if overflow:
    construct Err(ArithmeticError.Overflow)
else:
    continue with sum
```

No exception table, unwinder, dispatcher, allocator, or general runtime is
required.

---

# Check elimination

The compiler removes checks proven unnecessary through:

```text
constant evaluation
range contracts
in-list contracts
branch refinement
assert refinement
loop induction
fixed lengths
fixed capacities
generic constant arguments
call-site specialization
whole-program analysis
```

Example:

```sec
if index >= 0 && index < values.Length {
    let value := values[index]
}
```

The bounds check may be removed in the true branch.

Example:

```sec
type Divisor int range 1..int.Max

fn Divide(value: int, divisor: Divisor) int {
    return value / divisor
}
```

The zero-divisor check is removed.

---

# `try` syntax

`try` does not require parentheses.

Canonical:

```sec
let total := try left + right * quantity
```

It applies to the complete following expression until the natural grammatical
boundary.

Conceptual grouping:

```sec
let total := try (left + (right * quantity))
```

Parentheses narrow scope:

```sec
let total := left + (try right * quantity)
```

Only the multiplication is fallible.

---

# `try` boundaries

Typical boundaries include:

```text
statement completion
comma separating arguments or elements
closing parenthesis
closing bracket
closing brace
match or select arrow
local try-handler block
```

Example:

```sec
Process(try left + right, customer)
```

`try` covers only the first argument.

Example:

```sec
let value := try left + right {
    Err(error) => return Err(error)
}
```

The handler belongs to the full expression.

---

# What `try` converts

Within its expression subtree, `try` converts language-defined checks into
fallible control flow.

```sec
let result := try a + b * c
```

Potential fallible points:

```text
b * c
a + product
```

Evaluation remains left to right.

The first runtime failure stops the expression and produces its error.

---

# What `try` does not do

`try` does not catch arbitrary panic from called functions.

```sec
let result := try Calculate() + value
```

Cases:

```text
Calculate returns Result
    its error may be propagated or handled

Calculate is noPanic
    the call introduces no panic effect

Calculate may panic
    the enclosing expression still has that panic effect
```

`try` is not exception handling.

It does not unwind or resume a panicked call.

---

# Error typing

Every fallible operation has one explicit error type.

Examples:

```text
checked arithmetic
    ArithmeticError

bounds access
    IndexError

contract construction
    ContractError

capacity growth
    CapacityError or AllocationError

reference generation validation
    ReferenceError
```

Exact canonical error unions belong to core and specialized rulebooks.

---

# Propagation rule

Naked `try` may propagate an error only when the error is assignable to the
enclosing function's declared error type.

```sec
fn Add(left: int, right: int) Result[int, ArithmeticError] {
    let total := try left + right
    return Ok(total)
}
```

---

# Incompatible error types

When the error does not fit the function's declared error type, local mapping is
required.

```sec
type InvoiceError union {
    Calculation(ArithmeticError)
}

fn Calculate(
    left: int,
    right: int,
) Result[int, InvoiceError] {
    let total := try left + right {
        Err(error) => return Err(InvoiceError.Calculation(error))
    }

    return Ok(total)
}
```

The compiler must not choose the wrapper variant.

---

# No inferred error unions

Sec does not infer hidden unions such as:

```text
ParseError | IndexError | ArithmeticError
```

from implementation details.

Public error contracts remain explicit.

---

# No automatic union widening

The compiler does not add variants to named unions when a new fallible
operation appears.

That would alter:

```text
type identity
layout
ABI
match exhaustiveness
serialization
documentation
callers
```

The programmer changes the type explicitly.

---

# No arbitrary implicit wrapping

Given:

```sec
type ImportError union {
    InvalidHeader(ParseError)
    InvalidRow(ParseError)
}
```

a `ParseError` cannot be wrapped automatically.

The correct variant is domain meaning, not type inference.

---

# Generic error propagation

Generic functions may preserve an explicit generic error type.

```sec
fn Transform[T, U, E](
    values: ref T[],
    transform: fn(T) Result[U, E],
) Result[list[U], E] {
    // ...
}
```

A function changing error domains requires an explicit mapper.

```sec
fn Transform[T, U, SourceError, TargetError](
    values: ref T[],
    transform: fn(T) Result[U, SourceError],
    mapError: fn(SourceError) TargetError,
) Result[list[U], TargetError] {
    // ...
}
```

---

# Arithmetic checks

Ordinary arithmetic is checked according to `operators.md`.

```sec
let sum := left + right
let product := left * right
```

Fallible:

```sec
let sum := try left + right
let product := try left * right
```

Failure produces `ArithmeticError`.

---

# Alternative arithmetic semantics

Wrapping and saturation are different mathematical operations.

They use explicit generic compiler-known or core functions.

Illustrative:

```sec
let wrapped := arithmetic.WrappingAdd(left, right)
let saturated := arithmetic.SaturatingMultiply(left, right)
```

`try` never selects these automatically.

---

# Widening

Widening is an explicit type decision.

```sec
let total := int128(quantity) * int128(price)
```

Tooling may suggest widening.

It must not silently change the type or ABI.

---

# Division and remainder

Potential failures:

```text
division by zero
remainder by zero
minimum signed value divided by -1
related fixed-width representation overflow
```

Ordinary:

```sec
let result := numerator / denominator
```

Fallible:

```sec
let result := try numerator / denominator
```

---

# Shift checks

Potential failures:

```text
negative count
count equal to or greater than bit width
signed left-shift overflow
other invalid conditions defined by operators.md
```

Fallible:

```sec
let shifted := try value << count
```

Masked or wrapping shift behavior requires a separate explicit operation.

---

# Index checks

Ordinary:

```sec
let value := values[index]
```

Fallible:

```sec
let value := try values[index]
```

Failure produces the canonical concrete error `IndexError.OutOfBounds`.

---

# Slice checks

Potential checks:

```text
start lower bound
end upper bound
start/end ordering
inclusive-end representability
valid storage origin
```

Fallible:

```sec
let part := try values[start..<end]
```

---

# Collections

Operations that may exceed capacity, allocate, or fail expose a fallible path.

Illustrative:

```sec
try values.Add(item)
```

No hidden global heap or mandatory collection runtime is assumed.

---

# Allocation

Allocation failure is an expected technical error for panic-free code.

```sec
let report := try allocator.New[Report]()
```

Allocators may be:

```text
arena
fixed region
caller-provided allocator
system allocator wrapper
custom embedded allocator
```

No mandatory heap runtime is introduced.

---

# Conversions

Potentially failing conversions use `try`.

```sec
let value := try int32(largeValue)
let runeValue := try rune(codePoint)
let percent := try Percent(raw)
```

Proven-safe conversions need no dynamic check.

Lossy semantics require distinct explicit operations.

---

# Type contracts

Named-type construction may be fallible.

```sec
type Percent int range 0..100

let percent := try Percent(raw)
```

Compile-time constants are validated during compilation.

Runtime values use ordinary branches and Result construction.

---

# Contextual `require` contract proposal

`require` is not globally reserved and is not part of the canonical Sec 0.1
contract grammar in `contracts.md`.

If a future rulebook introduces a named-type predicate form, it may use
`require` contextually while preserving ordinary identifier uses. Illustrative
direction:

```sec
type InvoiceNumber string
    require IsValidInvoiceNumber
```

The exact grammar requires an explicit update to `contracts.md` before this form
becomes normative.

A require predicate must be:

```text
pure
deterministic
transitively noPanic
non-blocking
free of observable side effects
normally allocation-free
```

Failure produces `ContractError`.

It does not require panic or a runtime library.

---

# `@noPanic`

Conceptual:

```sec
@noPanic
fn Calculate(...) Result[Money, CalculateError] {
}
```

`@noPanic` is a compiler-verified transitive effect.

The compiler checks the reachable call graph.

---

# Ordinary operations in no-panic code

An ordinary panic-capable operation is permitted only when the compiler proves
failure impossible.

```sec
type Index4 int range 0..3

@noPanic
fn Read(values: int[4], index: Index4) int {
    return values[index]
}
```

Otherwise use `try` or an explicit total semantic.

---

# No-panic diagnostics

Diagnostics show the complete cause chain.

```text
CalculateInvoiceTotal is not panic-free
  -> calls CalculateLine
  -> evaluates quantity * unitPrice
  -> signed multiplication may overflow
```

Possible help:

```text
use `try`
add a local handler
map ArithmeticError
constrain operand types
widen explicitly
choose saturation
choose wrapping
```

The compiler does not choose business semantics.

---

# Defer and destructors

Every defer body is implicitly transitively noPanic.

Every destructor is transitively noPanic.

This is a compile-time requirement.

Invalid when overflow cannot be disproven:

```sec
defer {
    total = total + value
}
```

Fallible cleanup handles errors locally.

```sec
defer {
    match resource.TryClose() {
        Ok() => {
        }

        Err(error) => {
            cleanupDiagnostics.Record(error)
        }
    }
}
```

Every operation in that path must also be noPanic.

---

# Cleanup affecting return values

When cleanup failure must affect the public result, perform it explicitly before
return rather than hiding it in defer.

```sec
let closeResult := resource.TryClose()

match closeResult {
    Ok() => return Ok(value)
    Err(error) => return Err(error)
}
```

---

# Internal effect detail

The compiler may track detailed causes:

```text
MayOverflowPanic
MayDivisionPanic
MayBoundsPanic
MayContractPanic
MayAssertPanic
MayExplicitPanic
MayForeignAbort
```

User-facing summaries may collapse them to:

```text
noPanic
may panic
```

---

# FFI

Foreign functions are not assumed panic-free or abort-free.

FFI rules must classify effects such as:

```text
may abort
may unwind
may allocate
may block
returns error code
```

Unknown foreign effects are incompatible with verified noPanic code.

Foreign unwinding may not cross Sec frames by default.

---

# Unsafe code

Unsafe does not disable checks automatically.

```sec
unsafe {
    let value := array[index]
}
```

remains checked unless an explicit unchecked primitive is used.

Unchecked primitives, if introduced, remain separate from ordinary `try`.

---

# Panic endpoint

A panic-capable check may lower to a minimal endpoint.

Conceptual:

```text
sec_panic(PanicInfo) noreturn
```

The symbol name is illustrative.

The endpoint may be:

```text
inlined
compiler-emitted locally
provided by the application
provided by a target package
replaced by a target trap
```

It does not imply a general Sec runtime.

---

# Panic-free build policy

A target may require panic freedom for selected entrypoints or all reachable
code.

Illustrative:

```toml
panic = "forbid"
```

Exact manifest syntax belongs elsewhere.

The compiler verifies the call graph.

---

# Panic-free migration analysis

Ordinary formatting never changes panic behavior.

An explicit semantic tool may produce:

```text
panic-risk report
panic-free migration plan
call-graph impact analysis
reviewable patch
```

Sema and call-graph analysis discover risks.

The formatter only prints an approved semantic patch canonically.

---

# Migration report

Each risk records:

```text
stable risk ID
source location
function
entrypoint call chain
risk category
current behavior
alternatives
affected signatures
automatic decision status
```

Example:

```text
PF000142

Function:
    CalculateInvoiceTotal

Risk:
    signed integer multiplication may overflow

Expression:
    quantity * unitPrice

Alternatives:
    propagate ArithmeticError
    map locally
    prove through contracts
    widen explicitly
    choose saturation
    choose wrapping

Automatic selection:
    none
```

---

# Migration policy

The tool must not automatically choose:

```text
propagation
local recovery
widening
saturation
wrapping
changed domain type
changed contract
changed transaction behavior
```

These choices have different semantics.

---

# Safe generated actions

After explicit request, tooling may:

```text
insert `try` when the error type is already compatible
generate a handler skeleton
generate an error-mapping skeleton
add noPanic only after verification
remove a proven redundant check
```

Every behavioral change remains reviewable.

---

# Optimization

The compiler may:

```text
fold checks
merge equivalent checks
hoist loop-invariant checks
use target overflow flags
use target traps
specialize generic checks
remove proven checks
combine range checks
```

Optimization preserves:

```text
left-to-right evaluation
first-failure behavior
error type
panic reason
source attribution where required
side effects
```

---

# First-failure behavior

For:

```sec
let result := try First() + Second()
```

1. Evaluate `First()`.
2. Stop if it returns `Err`.
3. Evaluate `Second()`.
4. Stop if it returns `Err`.
5. Perform addition.
6. Return arithmetic error if it fails.
7. Otherwise produce the result.

Sequential errors are not aggregated.

---

# Diagnostics

Suggested diagnostic families:

```text
runtime-check.unhandled-overflow-risk
runtime-check.unhandled-division-risk
runtime-check.unhandled-bounds-risk
runtime-check.unhandled-contract-risk
runtime-check.incompatible-error-propagation
runtime-check.implicit-error-union-forbidden
runtime-check.defer-may-panic
runtime-check.destructor-may-panic
runtime-check.no-panic-call-chain
runtime-check.foreign-effect-unknown
runtime-check.migration-choice-required
```

Final IDs belong to the diagnostics registry.

---

# Tests

Required files:

```text
runtime_checks_valid.sec
runtime_checks_invalid.sec
runtime_checks_arithmetic_valid.sec
runtime_checks_arithmetic_invalid.sec
runtime_checks_bounds_valid.sec
runtime_checks_bounds_invalid.sec
runtime_checks_contracts_valid.sec
runtime_checks_contracts_invalid.sec
runtime_checks_try_valid.sec
runtime_checks_try_invalid.sec
runtime_checks_no_panic_valid.sec
runtime_checks_no_panic_invalid.sec
runtime_checks_defer_valid.sec
runtime_checks_defer_invalid.sec
```

---

# Lowering and binary tests

Verify:

```text
no mandatory runtime symbol in panic-free binaries
check removed when proven
inline fallible branch for `try`
correct Result error
direct target check where available
panic endpoint only for panic-capable path
no exception table required
no hidden allocation
left-to-right first failure
```

Build minimal programs for:

```text
linux amd64
linux arm64
linux arm32
freestanding
bare metal
```

A panic-free program must not require a Sec runtime library.

A panic-capable program requires only the selected endpoint or trap.

---

# Required synchronization

## Errorhandling revision-2 integration

A protected expression may contain several fallible sources with distinct
concrete error types. The compiler tracks these as an internal failure set;
this analysis metadata is not an inferred source union or public error type.

Every unhandled failure propagated by naked `try` must be assignable to the
enclosing Result error channel. Concrete Sec errors may widen to compiler-known
`error`; the compiler never invents a user-union wrapper.

Local try-handler lists are partial. Unmatched compatible failures propagate.
`Err(_)` may handle a heterogeneous set without binding it, while `Err(name)`
requires one already resolved binding type and cannot force inference of a
hidden common type. Failures from a guard or handler body are outside the
protected failure set and require their own handling. Option `None` remains
ordinary absence under `errorhandling.md`.

This document must remain synchronized with:

```text
panic.md
operators.md
grammar.md
errorhandling.md
default_values.md
contracts.md or variables_contracts.txt
types.md
collections.md
shaped-types.md
references.txt
ownership.md
copy_move.md
defer.md
discard.md
destruction.txt
allocation.txt
unsafe.md
platform/ffi.md
attributes.md
compiler_pipeline.txt
semantic_ir.txt
formatter.md
lsp.md
diagnostics.txt
build rules
language-rulebook-status.md
rules_implementations.txt
```

---

# Appendix A — Codex implementation plan

## A.1 Add rulebook

Add:

```text
rules/errors/runtime_checks.md
```

Update status and implementation trackers.

## A.2 Represent checks

Add explicit check categories in Sema or Semantic IR.

Conceptual:

```go
type CheckKind int

const (
    CheckOverflow CheckKind = iota
    CheckDivision
    CheckShift
    CheckBounds
    CheckSlice
    CheckConversion
    CheckContract
    CheckReferenceGeneration
    CheckAssertion
    CheckUnreachable
)
```

## A.3 Try-check mode

The expression analyzer carries a check mode such as:

```text
PanicCapable
Fallible
ProvenOnly
```

`try` applies Fallible mode to language-defined checks in its expression
subtree.

Function calls retain declared effects.

## A.4 Error compatibility

For naked propagation:

1. resolve produced error type;
2. resolve enclosing function error type;
3. test assignability;
4. propagate only if compatible;
5. otherwise require local mapping.

Do not infer unions.

## A.5 Proof queries

Implement proof for:

```text
overflow
zero divisor
shift range
bounds
slice range
contract predicate
reference generation
```

## A.6 Semantic IR

Represent:

```text
checked operation
fallible checked operation
proven operation
panic endpoint
error construction
```

## A.7 Direct lowering

Prefer:

```text
target intrinsic
ordinary compare
ordinary branch
local helper
user handler
target trap
```

No general runtime dependency.

## A.8 No-panic effect

Compute transitive status across:

```text
functions
methods
lambdas
defer bodies
destructors
require predicates
foreign declarations
```

## A.9 Migration analyzer

Build semantic reports outside the formatter.

Use stable risk IDs.

## A.10 Tests

Add proof, lowering, link-dependency, call-graph, and migration-report tests.

---

# Design summary

Sec uses checked semantics without requiring a language runtime.

Checks are removed when proven unnecessary.

Ordinary unproven checks may panic.

`try` converts language-defined checks in the following expression into explicit
fallible control flow.

`try` does not require parentheses.

`try` does not catch arbitrary panic from function calls.

Each fallible operation has an explicit error type.

Propagation requires error-type compatibility.

Sec does not infer error unions, widen named unions, or choose wrapper variants.

Every runtime failure has a panic-free path.

`@noPanic` is transitively verified.

Defer bodies and destructors are always noPanic.

No check requires exceptions, stack unwinding, hidden allocation, or a mandatory
runtime library.

Semantic tooling may plan panic-free migrations, but business and arithmetic
policy choices remain explicit.
