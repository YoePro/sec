# Runtime Checks

- **Status:** Normative
- **Created:** 2026-08-04
- **Last updated:** 2026-11-07 (stable chapter, subsection, and paragraph numbering); 2026-11-01 (string materialization failure modes, MD-039); 2026-10-28 (decimal division and arithmetic failures, MD-066/MD-067)
- **Document revision:** 1.1
- **Sec language version:** 0.1
- **Canonical path:** `rules/errors/runtime_checks.md`
- **Replaces:** Earlier unversioned revision at the same canonical path
- **Repository baseline reviewed:** `28780c01d63c3fc9eec2321ef67e3dd69962f043`
- **Implementation governance:** `governance/errors.yaml`
- **Related rulebooks:** `rules/errors/panic.md`, `rules/errors/errorhandling.md`, `rules/foundations/operators.md`, `rules/types/contracts.md`, `rules/types/default_values.md`, `rules/memory/ownership.md`, `rules/memory/copy_move.md`, `rules/memory/destruction.md`, `rules/memory/allocation.md`, `rules/memory/unsafe.md`, `rules/platform/ffi.md`, `rules/foundations/attributes.md`, `rules/compiler/semantic_ir.md`

---

## § 1 Status

**§ 1(1)** This document is the canonical runtime-check rulebook for Sec.

**§ 1(2)** The word `runtime` in this filename means:

```text
the check is evaluated while the generated program executes
```

**§ 1(3)** It does not mean:

```text
a mandatory Sec runtime library
a garbage-collected runtime
a hidden exception runtime
a scheduler dependency
a required process-wide support layer
```

**§ 1(4)** Sec must remain capable of producing programs without a language runtime.

**§ 1(5)** Runtime checks must lower through one or more of:

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

**§ 1(6)** No check may require linking a general Sec runtime.

---

## § 2 Purpose

**§ 2(1)** Sec uses checked language semantics.

**§ 2(2)** Operations such as:

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

**§ 2(3)** may require dynamic validation when static proof is insufficient.

**§ 2(4)** The language must provide:

1. a checked ordinary form;
2. a panic-free fallible form;
3. explicit alternative semantics where the mathematical operation differs;
4. static proof and check elimination;
5. compiler-visible panic effects;
6. no mandatory runtime dependency.

---

## § 3 Core rule

**§ 3(1)** When a language-defined operation failure is reached during semantic CTE, its
owning numeric/index/contract/check rule supplies the exact semantic reason and
the evaluator classifies the request as `SemanticOperationFailed`. Static
proof and optional check elimination do not themselves create a required CTE
context.

> Every language-defined runtime failure must have a path that does not panic.

**§ 3(2)** The non-panicking path may be:

```text
statically proven safe
fallible through `try`
expressed as an explicit Result-producing operation
expressed through an explicit alternative semantic such as wrapping or
saturation
```

**§ 3(3)** The compiler must not force panic as the only available behavior for a
foreseeable runtime condition.

---

## § 4 Runtime-check categories

### § 4.1 Arithmetic

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

**§ 4.1(1)** Decimal arithmetic failures use `ArithmeticError` with the most specific
applicable variant (`ScaleOverflow`, `PrecisionExhausted`, otherwise
`Overflow`; `rules/foundations/operators.md` "Decimal arithmetic failures").
An inexact decimal quotient rounded by "Decimal division" is a successful
value, not a runtime-check failure (MD-066/MD-067, 2026-10-28).

### § 4.2 Bounds and shape

```text
array index outside bounds
slice index outside bounds
invalid slice range
bounded collection capacity exceeded
vector or matrix shape mismatch
tensor dimension mismatch
spread length mismatch where dynamically checked
```

### § 4.3 Contracts

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

### § 4.4 References and representation

```text
invalid generation
expired arena reference
invalid optional-reference unwrap
invalid enum representation from foreign or unsafe input
invalid union tag from foreign or unsafe input
invalid scalar conversion
invalid ABI decode
```

**§ 4.4(1)** Ordinary ownership violations that are statically provable remain compile-time
errors.

### § 4.5 Assertions and control flow

```text
assertion failure
checked unreachable reached
explicit panic
```

**§ 4.5(1)** These are specified primarily by `panic.md`.

---

## § 5 Three compiler outcomes

### § 5.1 Proven safe

**§ 5.1(1)** The compiler proves the failure condition impossible.

```sec
type Small int range 0..100

fn Increment(value: Small) int {
    return value + 1
}
```

**§ 5.1(2)** When the result is representable, no runtime check or panic effect remains.

### § 5.2 Fallible

**§ 5.2(1)** The programmer uses `try`.

```sec
let total := try left + right
```

**§ 5.2(2)** The check becomes explicit control flow producing a typed error.

### § 5.3 Panic-capable

**§ 5.3(1)** The ordinary operation is used and failure cannot be disproven.

```sec
let total := left + right
```

**§ 5.3(2)** The operation has a panic effect.

**§ 5.3(3)** If failure occurs, `panic.md` defines the selected endpoint and containment.

---

## § 6 No mandatory runtime architecture

**§ 6(1)** Checked addition may lower conceptually to:

```text
sum, overflow = target_checked_add(left, right)

if overflow:
    panic endpoint
else:
    continue with sum
```

**§ 6(2)** A target may implement this through:

```text
overflow flag plus branch
comparison plus branch
target trap
direct call to a non-returning application handler
small local compiler-emitted helper
```

**§ 6(3)** Fallible lowering may be:

```text
sum, overflow = target_checked_add(left, right)

if overflow:
    construct Err(ArithmeticError.Overflow)
else:
    continue with sum
```

**§ 6(4)** No exception table, unwinder, dispatcher, allocator, or general runtime is
required.

---

## § 7 Check elimination

**§ 7(1)** The compiler removes checks proven unnecessary through:

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

**§ 7(2)** Example:

```sec
if index >= 0 && index < values.Length {
    let value := values[index]
}
```

**§ 7(3)** The bounds check may be removed in the true branch.

**§ 7(4)** Example:

```sec
type Divisor int range 1..int.Max

fn Divide(value: int, divisor: Divisor) int {
    return value / divisor
}
```

**§ 7(5)** The zero-divisor check is removed.

---

## § 8 `try` syntax

**§ 8(1)** `try` does not require parentheses.

**§ 8(2)** Canonical:

```sec
let total := try left + right * quantity
```

**§ 8(3)** It applies to the complete following expression until the natural grammatical
boundary.

**§ 8(4)** Conceptual grouping:

```sec
let total := try (left + (right * quantity))
```

**§ 8(5)** Parentheses narrow scope:

```sec
let total := left + (try right * quantity)
```

**§ 8(6)** Only the multiplication is fallible.

---

## § 9 `try` boundaries

**§ 9(1)** Typical boundaries include:

```text
statement completion
comma separating arguments or elements
closing parenthesis
closing bracket
closing brace
match or select arrow
local try-handler block
```

**§ 9(2)** Example:

```sec
Process(try left + right, customer)
```

**§ 9(3)** `try` covers only the first argument.

**§ 9(4)** Example:

```sec
let value := try left + right {
    Err(error) => return Err(error)
}
```

**§ 9(5)** The handler belongs to the full expression.

**§ 9(6)** The protected expression extends through its natural grammatical boundary and
includes call receivers, call arguments, operators, indexing, and every
successive member or call of a method chain (`errorhandling.md` § 13.1; MD-028,
2026-10-12):

```sec
let item := try Build(left + right, values[index])
```

**§ 9(7)** protects the addition, the index, and `Build`.

---

## § 10 What `try` converts

**§ 10(1)** Within its expression subtree, `try` converts language-defined checks into
fallible control flow.

```sec
let result := try a + b * c
```

**§ 10(2)** Potential fallible points:

```text
b * c
a + product
```

**§ 10(3)** Evaluation remains left to right.

**§ 10(4)** The first runtime failure stops the expression and produces its error.

---

## § 11 What `try` does not do

**§ 11(1)** `try` does not catch arbitrary panic from called functions.

```sec
let result := try Calculate() + value
```

**§ 11(2)** Cases:

```text
Calculate returns Result
    its error may be propagated or handled

Calculate is noPanic
    the call introduces no panic effect

Calculate may panic
    the enclosing expression still has that panic effect
```

**§ 11(3)** `try` is not exception handling.

**§ 11(4)** It does not unwind or resume a panicked call.

---

## § 12 Error typing

**§ 12(1)** Every fallible operation has one explicit error type.

**§ 12(2)** Examples:

```text
checked arithmetic
    ArithmeticError

bounds access
    IndexError

checked conversion, including contract construction
    ConversionError (a contract failure carries ContractError)

capacity growth
    CapacityError or AllocationError
```

**§ 12(3)** Exact canonical error unions belong to core and specialized rulebooks.

**§ 12(4)** Reference-generation validation is not an ordinary fallible operation. It has
three distinct outcomes:

```text
proven safe:
    no runtime check

ordinary direct safe reference (ref T / ref mut T), dynamic validation:
    check + panic.invalid-reference-generation on the violated safety guarantee

fallible stable/weak handle resolution:
    explicit Option/Result according to the handle API
```

**§ 12(5)** `try` does not recover a stale ordinary direct reference, and programmers do not
write `try` around ordinary safe dereferences. Generation/epoch validity is a
partial safety mechanism: it never replaces ownership, borrow compatibility,
bounds, initialization, type validity, relocation correctness, address-space
correctness, or concurrency synchronization, and it is not a runtime borrow
checker. An epoch dependency implies no mandatory runtime metadata
(`rules/memory/reference_model.md`; SEC-MLIR-P15 reference synchronization).

---

## § 13 Propagation rule

**§ 13(1)** Naked `try` may propagate an error only when the error is assignable to the
enclosing function's declared error type.

```sec
fn Add(left: int, right: int) Result[int, ArithmeticError] {
    let total := try left + right
    return Ok(total)
}
```

---

## § 14 Incompatible error types

**§ 14(1)** When the error does not fit the function's declared error type, local mapping is
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

**§ 14(2)** The compiler must not choose the wrapper variant.

---

## § 15 No inferred error unions

**§ 15(1)** Sec does not infer hidden unions such as:

```text
ParseError | IndexError | ArithmeticError
```

**§ 15(2)** from implementation details.

**§ 15(3)** Public error contracts remain explicit.

---

## § 16 No automatic union widening

**§ 16(1)** The compiler does not add variants to named unions when a new fallible
operation appears.

**§ 16(2)** That would alter:

```text
type identity
layout
ABI
match exhaustiveness
serialization
documentation
callers
```

**§ 16(3)** The programmer changes the type explicitly.

---

## § 17 No arbitrary implicit wrapping

**§ 17(1)** Given:

```sec
type ImportError union {
    InvalidHeader(ParseError)
    InvalidRow(ParseError)
}
```

**§ 17(2)** a `ParseError` cannot be wrapped automatically.

**§ 17(3)** The correct variant is domain meaning, not type inference.

---

## § 18 Generic error propagation

**§ 18(1)** Generic functions may preserve an explicit generic error type.

```sec
fn Transform[T, U, E](
    values: ref T[],
    transform: fn(T) Result[U, E],
) Result[list[U], E] {
    // ...
}
```

**§ 18(2)** A function changing error domains requires an explicit mapper.

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

## § 19 Arithmetic checks

**§ 19(1)** Ordinary arithmetic is checked according to `operators.md`.

```sec
let sum := left + right
let product := left * right
```

**§ 19(2)** Fallible:

```sec
let sum := try left + right
let product := try left * right
```

**§ 19(3)** Failure produces `ArithmeticError`.

---

## § 20 Alternative arithmetic semantics

**§ 20(1)** Wrapping and saturation are different mathematical operations.

**§ 20(2)** They use explicit generic compiler-known or core functions.

**§ 20(3)** Illustrative:

```sec
let wrapped := arithmetic.WrappingAdd(left, right)
let saturated := arithmetic.SaturatingMultiply(left, right)
```

**§ 20(4)** `try` never selects these automatically.

---

## § 21 Widening

**§ 21(1)** Widening is an explicit type decision.

```sec
let total := int128(quantity) * int128(price)
```

**§ 21(2)** Tooling may suggest widening.

**§ 21(3)** It must not silently change the type or ABI.

---

## § 22 Division and remainder

**§ 22(1)** Potential failures:

```text
division by zero
remainder by zero
minimum signed value divided by -1
related fixed-width representation overflow
```

**§ 22(2)** Ordinary:

```sec
let result := numerator / denominator
```

**§ 22(3)** Fallible:

```sec
let result := try numerator / denominator
```

**§ 22(4)** A decimal divisor that the compiler proves to be zero is a compile-time error,
with or without `try`; compilation stops. An unproven decimal divisor is
checked at run time: the ordinary operation panics with reason
`DivisionByZero`, and `try` yields `ArithmeticError.DivisionByZero`. This
decimal rule does not make every other provably failing runtime check a
compile-time error (`rules/foundations/operators.md` "Decimal division and
remainder by zero"; MD-066, 2026-10-28).

---

## § 23 Shift checks

**§ 23(1)** Potential failures:

```text
negative count
count equal to or greater than bit width
signed left-shift overflow
other invalid conditions defined by operators.md
```

**§ 23(2)** Fallible:

```sec
let shifted := try value << count
```

**§ 23(3)** Masked or wrapping shift behavior requires a separate explicit operation.

---

## § 24 Index checks

**§ 24(1)** Ordinary:

```sec
let value := values[index]
```

**§ 24(2)** Fallible:

```sec
let value := try values[index]
```

**§ 24(3)** Failure produces the canonical concrete error `IndexError.OutOfBounds`.

---

## § 25 Slice checks

**§ 25(1)** Slices require explicit reference construction. Fallible forms include:

```sec
let part := try ref values[start..<end]
let writable := try ref mut values[start..<end]
```

**§ 25(2)** `try (ref values[start..<end])` is equivalent. A bare range expression does
not create a slice. Source, start and end expressions are evaluated once in
source order. For an existing dynamically validated slice, reference
generation/provenance validation follows operand evaluation and precedes
spatial range/index validation.

**§ 25(3)** Canonical range normalization is `[start, endExclusive)`: omitted start is
zero, omitted end is source length, and inclusive end is incremented only
after validation, with exact arithmetic. Compile-time-invalid ranges are
compile-time errors. Runtime spatial failure follows the ordered core mapping
in `rules/collections/collections.md` § 7.5:

1. Invalid/negative endpoints → `RangeError.InvalidRange`.
2. Explicit start after explicit represented end → `RangeError.StartAfterEnd`.
3. Endpoints beyond the allowed source bounds → `RangeError.OutOfBounds`.

**§ 25(4)** Otherwise unrepresentable normalization is `RangeError.InvalidRange`; valid
wide values must not be rejected because the compiler host has a narrower
integer type. Optimization preserves the selected error identity.

**§ 25(5)** Ordinary range failure uses `panic.bounds` with `slice-range` provenance;
`try` converts the spatial failure to `RangeError`. Slice indexing uses
`IndexError.OutOfBounds` on its fallible path, or `panic.bounds` with
`slice-index` provenance otherwise. There is no separate `BoundsError`.

**§ 25(6)** A stale ordinary direct slice takes `panic.invalid-reference-generation`.
Neither fallible range creation nor `try staleSlice[index]` catches that
failure as `RangeError` or `IndexError`. `unsafe` does not remove these safe
slice bounds or validity obligations.

**§ 25(7)** String ranges (MD-126, `rules/corrections/applied/MD-126-borrowed-string-ranges-rune-iteration-applied-20261105.md`) follow the same contract: `text[a..<b]` over a
`string` or `StringView` evaluates source, start and end once in source order,
normalizes rune endpoints to `[start, endExclusive)`, and maps dynamic failure
in the same order to `RangeError.InvalidRange`, `RangeError.StartAfterEnd` and
`RangeError.OutOfBounds`. The ordinary form uses `panic.bounds` with
`slice-range` provenance; `try text[a..<b]` yields `RangeError`. No new error
type or mid-scalar variant exists, since rune endpoints are scalar
boundaries. The string form needs no written `ref`.

---

## § 26 Collections

**§ 26(1)** Operations that may exceed capacity, allocate, or fail expose a fallible path.

**§ 26(2)** Illustrative:

```sec
try values.Add(item)
```

**§ 26(3)** No hidden global heap or mandatory collection runtime is assumed.

---

## § 27 Allocation

**§ 27(1)** Allocation failure is an expected technical error for panic-free code.

```sec
let report := try allocator.New[Report]()
```

**§ 27(2)** Allocators may be:

```text
arena
fixed region
caller-provided allocator
system allocator wrapper
custom embedded allocator
```

**§ 27(3)** No mandatory heap runtime is introduced.

**§ 27(4)** Runtime string `+` and `+=` are the exception: without `try` a materialization
failure panics with `StringMaterializationFailure`, and with `try` it is
`StringError.Allocation` (MD-039, `rules/foundations/operators.md`).

---

## § 28 Conversions

**§ 28(1)** Potentially failing conversions use `try`.

```sec
let value := try int32(largeValue)
let runeValue := try rune(codePoint)
let percent := try Percent(raw)
```

**§ 28(2)** Proven-safe conversions need no dynamic check.

**§ 28(3)** A runtime-dependent checked conversion that can fail uses the core error type
`ConversionError`; an intrinsic destination-domain failure takes precedence over
declared contracts, and the first violated contract in source order is reported
as `ConversionError.Contract(ContractError.Violation { ... })`
(`rules/types/contracts.md`, "Public conversion and contract errors").

**§ 28(4)** Lossy semantics require distinct explicit operations.

---

## § 29 Type contracts

**§ 29(1)** Named-type construction may be fallible.

```sec
type Percent int range 0..100

let percent := try Percent(raw)
```

**§ 29(2)** Compile-time constants are validated during compilation.

**§ 29(3)** Runtime values use ordinary branches and Result construction.

---

## § 30 Contextual `require` contract proposal

**§ 30(1)** `require` is not globally reserved and is not part of the canonical Sec 0.1
contract grammar in `rules/types/contracts.md`.

**§ 30(2)** If a future rulebook introduces a named-type predicate form, it may use
`require` contextually while preserving ordinary identifier uses. Illustrative
direction:

```sec
type InvoiceNumber string
    require IsValidInvoiceNumber
```

**§ 30(3)** The exact grammar requires an explicit update to `rules/types/contracts.md` before this form
becomes normative.

**§ 30(4)** A require predicate must be:

```text
pure
deterministic
transitively noPanic
non-blocking
free of observable side effects
normally allocation-free
```

**§ 30(5)** Failure produces `ContractError`.

**§ 30(6)** It does not require panic or a runtime library.

---

## § 31 `@noPanic`

**§ 31(1)** Conceptual:

```sec
@noPanic
fn Calculate(...) Result[Money, CalculateError] {
}
```

**§ 31(2)** `@noPanic` is a compiler-verified transitive effect.

**§ 31(3)** The compiler checks the reachable call graph.

---

## § 32 Ordinary operations in no-panic code

**§ 32(1)** An ordinary panic-capable operation is permitted only when the compiler proves
failure impossible.

```sec
type Index4 int range 0..3

@noPanic
fn Read(values: int[4], index: Index4) int {
    return values[index]
}
```

**§ 32(2)** Otherwise use `try` or an explicit total semantic.

---

## § 33 No-panic diagnostics

**§ 33(1)** Diagnostics show the complete cause chain.

```text
CalculateInvoiceTotal is not panic-free
  -> calls CalculateLine
  -> evaluates quantity * unitPrice
  -> signed multiplication may overflow
```

**§ 33(2)** Possible help:

```text
use `try`
add a local handler
map ArithmeticError
constrain operand types
widen explicitly
choose saturation
choose wrapping
```

**§ 33(3)** The compiler does not choose business semantics.

---

## § 34 Defer and destructors

**§ 34(1)** Every defer body is implicitly transitively noPanic.

**§ 34(2)** Every destructor is transitively noPanic.

**§ 34(3)** This is a compile-time requirement.

**§ 34(4)** Invalid when overflow cannot be disproven:

```sec
defer {
    total = total + value
}
```

**§ 34(5)** Fallible cleanup handles errors locally.

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

**§ 34(6)** Every operation in that path must also be noPanic.

---

## § 35 Cleanup affecting return values

**§ 35(1)** When cleanup failure must affect the public result, perform it explicitly before
return rather than hiding it in defer.

```sec
let closeResult := resource.TryClose()

match closeResult {
    Ok() => return Ok(value)
    Err(error) => return Err(error)
}
```

---

## § 36 Internal effect detail

**§ 36(1)** The compiler may track detailed causes:

```text
MayOverflowPanic
MayDivisionPanic
MayBoundsPanic
MayContractPanic
MayAssertPanic
MayExplicitPanic
MayForeignAbort
```

**§ 36(2)** User-facing summaries may collapse them to:

```text
noPanic
may panic
```

---

## § 37 FFI

**§ 37(1)** Foreign functions are not assumed panic-free or abort-free.

**§ 37(2)** FFI rules must classify effects such as:

```text
may abort
may unwind
may allocate
may block
returns error code
```

**§ 37(3)** Unknown foreign effects are incompatible with verified noPanic code.

**§ 37(4)** Foreign unwinding may not cross Sec frames by default.

---

## § 38 Unsafe code

**§ 38(1)** Unsafe does not disable checks automatically.

```sec
unsafe {
    let value := array[index]
}
```

**§ 38(2)** remains checked unless an explicit unchecked primitive is used.

**§ 38(3)** Unchecked primitives, if introduced, remain separate from ordinary `try`.

---

## § 39 Panic endpoint

**§ 39(1)** A panic-capable check may lower to a minimal endpoint.

**§ 39(2)** Conceptual:

```text
sec_panic(PanicInfo) noreturn
```

**§ 39(3)** The symbol name is illustrative.

**§ 39(4)** The endpoint may be:

```text
inlined
compiler-emitted locally
provided by the application
provided by a target package
replaced by a target trap
```

**§ 39(5)** It does not imply a general Sec runtime.

---

## § 40 Panic-free build policy

**§ 40(1)** A target may require panic freedom for selected entrypoints or all reachable
code.

**§ 40(2)** Illustrative:

```toml
panic = "forbid"
```

**§ 40(3)** Exact manifest syntax belongs elsewhere.

**§ 40(4)** The compiler verifies the call graph.

---

## § 41 Panic-free migration analysis

**§ 41(1)** Ordinary formatting never changes panic behavior.

**§ 41(2)** An explicit semantic tool may produce:

```text
panic-risk report
panic-free migration plan
call-graph impact analysis
reviewable patch
```

**§ 41(3)** Sema and call-graph analysis discover risks.

**§ 41(4)** The formatter only prints an approved semantic patch canonically.

---

## § 42 Migration report

**§ 42(1)** Each risk records:

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

**§ 42(2)** Example:

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

## § 43 Migration policy

**§ 43(1)** The tool must not automatically choose:

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

**§ 43(2)** These choices have different semantics.

---

## § 44 Safe generated actions

**§ 44(1)** After explicit request, tooling may:

```text
insert `try` when the error type is already compatible
generate a handler skeleton
generate an error-mapping skeleton
add noPanic only after verification
remove a proven redundant check
```

**§ 44(2)** Every behavioral change remains reviewable.

---

## § 45 Optimization

**§ 45(1)** The compiler may:

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

**§ 45(2)** Optimization preserves:

```text
left-to-right evaluation
first-failure behavior
error type
panic reason
source attribution where required
side effects
```

---

## § 46 First-failure behavior

**§ 46(1)** For:

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

**§ 46(2)** Sequential errors are not aggregated.

---

## § 47 Diagnostics

**§ 47(1)** Suggested diagnostic families:

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

**§ 47(2)** Final IDs belong to the diagnostics registry.

---

## § 48 Tests

**§ 48(1)** Required files:

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

## § 49 Lowering and binary tests

**§ 49(1)** Verify:

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

**§ 49(2)** Build minimal programs for:

```text
linux amd64
linux arm64
linux arm32
freestanding
bare metal
```

**§ 49(3)** A panic-free program must not require a Sec runtime library.

**§ 49(4)** A panic-capable program requires only the selected endpoint or trap.

---

## § 50 Required synchronization

### § 50.1 Errorhandling revision-2 integration

**§ 50.1(1)** A protected expression may contain several fallible sources with distinct
concrete error types. The compiler tracks these as an internal failure set;
this analysis metadata is not an inferred source union or public error type.

**§ 50.1(2)** Every unhandled failure propagated by naked `try` must be assignable to the
enclosing Result error channel. Concrete Sec errors may widen to compiler-known
`error`; the compiler never invents a user-union wrapper.

**§ 50.1(3)** Local try-handler lists are partial. Unmatched compatible failures propagate.
`Err(_)` may handle a heterogeneous set without binding it, while `Err(name)`
requires one already resolved binding type and cannot force inference of a
hidden common type. Failures from a guard or handler body are outside the
protected failure set and require their own handling. Option `None` remains
ordinary absence under `errorhandling.md`.

**§ 50.1(4)** This document must remain synchronized with:

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
references.md
ownership.md
copy_move.md
defer.md
discard.md
destruction.md
allocation.md
unsafe.md
platform/ffi.md
attributes.md
compiler_pipeline.md
semantic_ir.md
formatter.md
lsp.md
diagnostics.md
build rules
language-rulebook-status.md
```

---

## § 51 Design summary

**§ 51(1)** Sec uses checked semantics without requiring a language runtime.

**§ 51(2)** Checks are removed when proven unnecessary.

**§ 51(3)** Ordinary unproven checks may panic.

**§ 51(4)** `try` converts language-defined checks in the following expression into explicit
fallible control flow.

**§ 51(5)** `try` does not require parentheses.

**§ 51(6)** `try` does not catch arbitrary panic from function calls.

**§ 51(7)** Each fallible operation has an explicit error type.

**§ 51(8)** Propagation requires error-type compatibility.

**§ 51(9)** Sec does not infer error unions, widen named unions, or choose wrapper variants.

**§ 51(10)** Every runtime failure has a panic-free path.

**§ 51(11)** `@noPanic` is transitively verified.

**§ 51(12)** Defer bodies and destructors are always noPanic.

**§ 51(13)** No check requires exceptions, stack unwinding, hidden allocation, or a mandatory
runtime library.

**§ 51(14)** Semantic tooling may plan panic-free migrations, but business and arithmetic
policy choices remain explicit.
