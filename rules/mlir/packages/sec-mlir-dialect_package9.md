# Sec MLIR Program - Implementation Package 9

## Package status

Implementation package for the Sec compiler.

Package ID: `SEC-MLIR-P9`\
Package title: `Typed Arithmetic Failure Flow`\
Repository: `https://github.com/YoePro/sec`\
Repository branch: `main`\
Repository sync commit used for this package: `152c772`\
Repository full sync commit: `152c7727d805b4e66349de48700c369fc70ef2db`\
Repository sync date: `2026-08-09`\
Semantic IR version: `1`\
Sec MLIR dialect schema before package: `4`\
Sec MLIR dialect schema after package: `5`\
Sec MLIR lowering specification after package: `5`

Package 9 gives checked integer failure a typed reason and implements the first
panic-free fallible arithmetic path:

```text
try <integer arithmetic expression>
```

with naked propagation to:

```text
Result[U, ArithmeticError]
```

It also preserves the ordinary panic-capable arithmetic path.

Local `try { Err(...) => ... }` handlers are deliberately deferred.

---

# 1. Normative authority

Implementation follows:

```text
rules/errors/runtime_checks.md
rules/errors/errorhandling.txt
rules/foundations/operators.md
rules/errors/panic.md
rules/library/core-library.md
rules/analysis/effect_analysis.md
rules/types/types.md
    ↓
rules/compiler/semantic_ir.txt
    ↓
rules/mlir/sec_mlir.md
    ↓
rules/mlir/sec_mlir_dialect.md
    ↓
rules/mlir/sec_mlir_lowering.md
    ↓
implementation package
    ↓
implementation
```

Before implementation:

1. update `rules/mlir/sec_mlir_dialect.md` with
   `sec_mlir_dialect_package9.md`;
2. update `rules/mlir/sec_mlir_lowering.md` with
   `sec_mlir_lowering_package9.md`;
3. apply the normative core error addition described by
   `sec_core_arithmetic_error_package9.md`.

Package 9 does not redefine ordinary integer arithmetic.

It defines how the already-checked failure is represented and routed.

---

# 2. Repository synchronization note

Package 9 is the first package in this sequence based on repository HEAD:

```text
152c772
```

rather than:

```text
d48035c
```

The new repository commit contains rulebook/package synchronization changes.

Package 9 therefore uses the current `main` versions of:

```text
runtime_checks.md
errorhandling.txt
panic.md
core-library.md
types.md
semantic_ir.txt
```

as its normative basis.

---

# 3. Required wide-builtin status invariant

These remain active Sec builtin types:

```text
int128
int256
uint128
uint256
decimal128
```

Package 9 must not introduce wording that marks them as future, planned,
reserved, placeholder, or not-yet-active.

Arithmetic `try` covers the active wide integer families:

```text
int128
int256
uint128
uint256
```

exactly like the smaller integer types.

---

# 4. Normative gap closed by Package 9

`runtime_checks.md` requires:

```text
checked arithmetic -> ArithmeticError
```

and gives:

```sec
fn Add(left: int, right: int) Result[int, ArithmeticError] {
    let total := try left + right
    return Ok(total)
}
```

The current core rulebook lists fundamental core error types but does not yet
define the exact minimum `ArithmeticError` value set.

Package 9 closes that gap.

Canonical minimum:

```sec
enum ArithmeticError {
    Overflow
    DivisionByZero
    InvalidShift
}
```

This error is always available from core.

It is allocation-free.

It is a normal exact named Sec error type.

No implicit conversion to other error types is introduced.

---

# 5. Arithmetic failure mapping

Canonical mapping:

```text
checked negation minimum-value failure
    -> ArithmeticError.Overflow

checked add overflow
    -> ArithmeticError.Overflow

checked subtract overflow/underflow
    -> ArithmeticError.Overflow

checked multiply overflow
    -> ArithmeticError.Overflow

division by zero
    -> ArithmeticError.DivisionByZero

remainder by zero
    -> ArithmeticError.DivisionByZero

signed minimum / -1
    -> ArithmeticError.Overflow

signed minimum % -1 checked failure
    -> ArithmeticError.Overflow

negative shift count
    -> ArithmeticError.InvalidShift

shift count >= value bit width
    -> ArithmeticError.InvalidShift

signed left-shift representability failure
    -> ArithmeticError.Overflow
```

This mapping is deterministic.

No implementation layer may choose a different variant.

---

# 6. Why the Package 7/8 bool-only failure model must evolve

Package 7 originally represented checked integer operations as:

```text
result
failed
```

That is sufficient for ordinary panic branching but insufficient for typed
fallible arithmetic.

Examples:

```text
signed division:
    zero divisor
    signed minimum / -1

signed left shift:
    invalid count
    representability overflow
```

Both cases share one `failed` bit but require different typed errors.

Package 9 therefore evolves compiler-generated checked integer semantics to:

```text
result
failed
reason
```

where:

```text
reason = None
       | Overflow
       | DivisionByZero
       | InvalidShift
```

---

# 7. Failure reason invariant

Define compiler-internal semantic reason:

```text
ArithmeticFailureReason.None
ArithmeticFailureReason.Overflow
ArithmeticFailureReason.DivisionByZero
ArithmeticFailureReason.InvalidShift
```

Rules:

```text
failed == false  <=> reason == None
failed == true   <=> reason != None
```

Compiler-generated checked arithmetic must obey this invariant.

The reason is not a user-visible replacement for `ArithmeticError`.

It is the semantic bridge used to choose the correct error or panic reason.

---

# 8. Failure priority

When more than one internal condition could be true because a safe substitute
computation is also performed, reason selection follows Sec semantic input
conditions, not artifacts of the substitute computation.

Priority rules:

```text
division/remainder:
    zero divisor -> DivisionByZero
    otherwise signed min/-1 -> Overflow

signed left shift:
    invalid count -> InvalidShift
    otherwise unrepresentable result -> Overflow
```

Safe substitute arithmetic must not create a competing reason.

---

# 9. Semantic IR changes

Semantic IR version remains:

```go
const Version uint32 = 1
```

Package 9 is an additive completion of the initial semantic model.

Add:

```text
ArithmeticFailureReason
CoreErrorType
ResultType
ResultOkOp
ResultErrOp
ArithmeticErrorFromReasonOp
```

Extend checked integer operations to expose:

```text
result
failed
reason
```

Do not bump Semantic IR version solely for adding operations within the declared
v1 semantic universe.

---

# 10. Semantic IR `ArithmeticFailureReason`

Recommended Go enum:

```go
type ArithmeticFailureReason string

const (
    ArithmeticFailureNone           ArithmeticFailureReason = "none"
    ArithmeticFailureOverflow       ArithmeticFailureReason = "overflow"
    ArithmeticFailureDivisionByZero ArithmeticFailureReason = "division-by-zero"
    ArithmeticFailureInvalidShift   ArithmeticFailureReason = "invalid-shift"
)
```

All code identifiers and strings are implementation-facing.

The source-level error type remains `ArithmeticError`.

---

# 11. Semantic IR checked operation result

Recommended common result record:

```go
type CheckedIntegerResult struct {
    Value  ValueID
    Failed ValueID
    Reason ValueID
}
```

or equivalent explicit result values on each operation.

Canonical types:

```text
Value  -> integer operand/result type
Failed -> bool
Reason -> ArithmeticFailureReason
```

The verifier checks the reason invariant structurally where possible and through
operation-specific validation.

---

# 12. Arithmetic failure block evolution

Package 7 failure blocks originally needed no block arguments.

Package 9 canonical failure blocks receive the reason:

```text
^failure(%reason: ArithmeticFailureReason):
    ...
```

The checked operation block branches:

```text
cond_br %failed,
    ^failure(%reason),
    ^success()
```

The failure reason therefore dominates its use and is explicit CFG data.

---

# 13. Ordinary arithmetic failure path

Ordinary checked arithmetic remains panic-capable.

Canonical Semantic IR:

```text
%result, %failed, %reason = checked add ...

cond_br %failed,
    ^failure(%reason),
    ^success()

^failure(%reason):
    arithmetic.fail %reason

^success:
    ...
```

`ArithmeticFailureOp` now consumes the reason.

It remains non-returning.

It remains a semantic panic-capable endpoint, not a fixed runtime call.

---

# 14. Fallible `try` arithmetic path

For:

```sec
fn Add(left: int, right: int) Result[int, ArithmeticError] {
    let total := try left + right
    return Ok(total)
}
```

canonical Semantic IR:

```text
%result, %failed, %reason = checked add %left, %right

cond_br %failed,
    ^try_failure(%reason),
    ^try_success()

^try_failure(%reason):
    %error = arithmetic.error.from_reason %reason
    %err_result = result.err %error
        : Result[int, ArithmeticError]
    return %err_result

^try_success:
    ...
    %ok_result = result.ok %result
        : Result[int, ArithmeticError]
    return %ok_result
```

The expression itself evaluates to the unwrapped success integer value in the
success continuation.

---

# 15. Naked `try` propagation scope

Package 9 supports only naked arithmetic `try` propagation:

```sec
let value := try left + right
let value := try left - right
let value := try left * right
let value := try left / right
let value := try left % right
let value := try -left
let value := try left << count
let value := try left >> count
```

Only operators that are actually fallible need `try`.

Package 9 does not add `try` around total:

```text
bitwise
comparison
unary plus
```

---

# 16. Enclosing function requirement

For Package 9 naked arithmetic propagation, the enclosing function must return:

```text
Result[U, ArithmeticError]
```

with exact `ArithmeticError` as the error type.

Do not automatically:

```text
wrap ArithmeticError into another union
widen an error union
select a user-defined union variant
convert to OverflowError
convert to DivisionByZeroError
```

If the function declares another error type, the frontend requires explicit
local mapping.

Local mapping is deferred to a later package.

---

# 17. General Result type foundation

Package 9 adds canonical Semantic IR support for:

```text
Result[T, E]
```

as a semantic type because naked arithmetic propagation returns an ordinary
Result value.

The type retains:

```text
success type identity
error type identity
must-use classification
```

No physical tag/payload layout is selected in Package 9.

---

# 18. Core error semantic type

Add a canonical semantic representation for compiler-known core error identity.

Recommended concept:

```text
CoreErrorType("core::ArithmeticError")
```

This preserves exact error identity without choosing an enum integer layout.

Do not represent `ArithmeticError` as a universal runtime error code.

Do not collapse it to `int`.

---

# 19. Result construction operations

Add:

```text
ResultOkOp
ResultErrOp
```

`ResultOkOp`:

```text
input:
    T

result:
    Result[T, E]
```

For `T=void`, support the canonical zero-value `Ok()` representation without a
value operand.

`ResultErrOp`:

```text
input:
    E

result:
    Result[T, E]
```

No hidden allocation.

No unwinding.

---

# 20. `ArithmeticErrorFromReasonOp`

Input:

```text
ArithmeticFailureReason
```

Output:

```text
core ArithmeticError
```

Mapping is exactly section 5.

`None` is invalid input to this operation.

Verifier must reject construction from `None`.

This operation is pure and allocation-free.

---

# 21. Sema resolved `try` fact

The Semantic IR builder must not reinterpret the presence of the `try` token.

Add or extend a read-only Sema fact:

```go
type ResolvedTryKind string

const (
    TryResultPropagation     ResolvedTryKind = "result-propagation"
    TryArithmeticPropagation ResolvedTryKind = "arithmetic-propagation"
    TryLocalHandler          ResolvedTryKind = "local-handler"
    TryAssignment            ResolvedTryKind = "assignment"
)
```

Recommended query:

```go
func (a *Analyzer) ResolvedTryOf(
    expr *ast.TryExpression,
) (ResolvedTry, bool)
```

Package 9 accepts only:

```text
TryArithmeticPropagation
```

in the new arithmetic path.

Other forms remain explicit unsupported Semantic IR features unless an earlier
package already supports them.

---

# 22. Sema must retain error type

Resolved arithmetic `try` metadata includes:

```text
failure error type = ArithmeticError
enclosing Result error type
propagation permitted = true/false
```

The builder must not search enclosing function syntax to reconstruct this.

It consumes Sema's resolved fact.

---

# 23. `try` does not catch arbitrary panic

Package 9 preserves the rule:

```text
try arithmetic converts the arithmetic check failure to ArithmeticError
```

It does not catch:

```text
panic from a called operand function
explicit panic
bounds panic in an operand
contract panic in an operand
foreign abort
```

Example:

```sec
let value := try Calculate() + other
```

If `Calculate()` itself may panic, that panic effect remains.

Only Result-returning call errors and the selected fallible arithmetic check are
part of `try` semantics.

Package 9 implements only the arithmetic check part.

---

# 24. Effect-analysis integration

Ordinary checked integer arithmetic whose safety cannot be proven contributes:

```text
MayArithmeticPanic
```

or the repository's equivalent detailed effect.

The same arithmetic expression under valid naked `try` propagation does not
contribute an arithmetic panic effect.

Instead it contributes explicit fallible error flow:

```text
ArithmeticError
```

Effects from evaluating operands remain.

This distinction must be visible to `@noPanic` verification.

---

# 25. `@noPanic` test

Valid conceptual source:

```sec
@noPanic
fn Add(left: int32, right: int32) Result[int32, ArithmeticError] {
    let sum := try left + right
    return Ok(sum)
}
```

is compatible with noPanic when:

```text
the only dynamic failure is the handled/propagated arithmetic check
and all operand evaluation is transitively noPanic.
```

Ordinary:

```sec
@noPanic
fn Add(left: int32, right: int32) int32 {
    return left + right
}
```

must still fail noPanic verification unless overflow is statically disproven.

---

# 26. Left-to-right and first-failure

For:

```sec
let result := try First() + Second()
```

preserve:

```text
1. evaluate First()
2. stop if First returns Err where that Result handling is implemented
3. evaluate Second()
4. stop if Second returns Err
5. perform checked addition
6. stop and propagate ArithmeticError if addition fails
7. otherwise produce result
```

Package 9 arithmetic lowering must not move arithmetic error construction before
operand evaluation.

---

# 27. Package 7 correction

Update the Package 7 checked operation contract.

Compiler-generated schema-v5 high-level checked integer ops return:

```text
result
failed
reason
```

Package 7/9 verifier checks:

```text
reason type is ArithmeticFailureReason
failed is i1
failure block receives reason
true branch passes reason to failure block
false branch does not consume reason
```

Schema-v4 two-result operations remain parseable only for regression/legacy
tests.

New compiler output must use schema v5.

---

# 28. Package 8 correction

Package 8 lowering must compute both:

```text
failed
reason
```

from the same safe total arithmetic conditions.

Required mappings:

```text
neg/add/sub/mul:
    failed ? Overflow : None

unsigned division/remainder:
    rhs == 0
        ? DivisionByZero
        : None

signed division/remainder:
    rhs == 0
        ? DivisionByZero
        : (min/-1 ? Overflow : None)

unsigned shifts:
    invalid count
        ? InvalidShift
        : None

signed right shift:
    invalid count
        ? InvalidShift
        : None

signed left shift:
    invalid count
        ? InvalidShift
        : (representability failure ? Overflow : None)
```

The reason SSA value replaces the checked-op reason result and remains passed to
the existing failure block.

---

# 29. Reason computation must be total

Reason selection may use:

```text
arith.constant
arith.cmpi
arith.andi
arith.ori
arith.select
```

or equivalent total operations.

It must not evaluate unsafe arithmetic merely to choose a reason.

Reason selection uses the already-safe conditions computed by Package 8.

---

# 30. Sec MLIR dialect schema version 5

Compiler-generated high-level Sec MLIR now uses:

```mlir
sec.dialect_version = 5 : i32
```

Schema versions 1-4 remain regression inputs.

Schema 5 adds:

```text
!sec.arithmetic_failure_reason
!sec.core_error<"identity">
!sec.result<T, E>

sec.arithmetic_error.from_reason
sec.result.ok
sec.result.err
```

and evolves checked integer operation/failure-block contracts.

---

# 31. `!sec.arithmetic_failure_reason`

Compiler-internal semantic type.

Possible values:

```text
none
overflow
division_by_zero
invalid_shift
```

It is not a public source type.

No physical integer representation is defined in schema v5.

---

# 32. Failure reason constants

Add:

```text
sec.arithmetic_failure_reason.constant
```

Required enum attribute:

```text
none
overflow
division_by_zero
invalid_shift
```

Result:

```text
!sec.arithmetic_failure_reason
```

This is useful for Package 8 reason selection and tests.

A later lowering may choose a compact integer representation.

---

# 33. `!sec.core_error<"identity">`

High-level type representing one exact compiler-known core error type whose lower
physical representation is not yet selected.

Package 9 compiler output uses:

```mlir
!sec.core_error<"core::ArithmeticError">
```

The identity string is opaque.

Verifier:

```text
non-empty identity
known core error identity when compiler-generated
```

Package 9 does not define a universal error base type.

---

# 34. `!sec.result<T, E>`

High-level semantic Result type.

Parameters:

```text
success: Type
error: Type
```

Rules:

```text
success may represent void through the dialect's canonical void-result mechanism
error must be a valid non-void type
Result identity includes both T and E
no physical tag/payload layout is implied
```

Package 9 does not lower Result representation.

---

# 35. `sec.arithmetic_error.from_reason`

Operand:

```text
!sec.arithmetic_failure_reason
```

Result:

```text
!sec.core_error<"core::ArithmeticError">
```

Verifier:

```text
result identity exactly core::ArithmeticError
```

Runtime/semantic verifier rejects a statically known `none` reason.

Mapping:

```text
overflow         -> ArithmeticError.Overflow
division_by_zero -> ArithmeticError.DivisionByZero
invalid_shift    -> ArithmeticError.InvalidShift
```

---

# 36. `sec.result.ok`

Operands:

```text
success value when T != void
```

Result:

```text
!sec.result<T, E>
```

Verifier:

```text
operand type exactly T
```

For `T=void`, use the schema-defined zero-operand form.

No allocation.

---

# 37. `sec.result.err`

Operand:

```text
error value E
```

Result:

```text
!sec.result<T, E>
```

Verifier:

```text
operand type exactly E
```

No implicit error conversion.

No automatic union wrapping.

---

# 38. Schema-v5 checked integer result contract

Compiler-generated checked operations produce:

```text
result: T
failed: i1
reason: !sec.arithmetic_failure_reason
```

For schema-v5 compiler output:

```text
failed false implies reason none
failed true implies non-none reason
```

The dedicated guard verifier validates the CFG relationship.

---

# 39. Schema-v5 failure block

Canonical MLIR:

```mlir
%result, %failed, %reason = "sec.int.binary_checked"(...)
    {kind = ...}
    : (...) -> (T, i1, !sec.arithmetic_failure_reason)

cf.cond_br %failed,
    ^failure(%reason : !sec.arithmetic_failure_reason),
    ^success
```

Ordinary:

```mlir
^failure(%reason: !sec.arithmetic_failure_reason):
    "sec.fail.arithmetic"(%reason)
        : (!sec.arithmetic_failure_reason) -> ()
```

Fallible:

```mlir
^failure(%reason: !sec.arithmetic_failure_reason):
    %error = "sec.arithmetic_error.from_reason"(%reason)
        : (!sec.arithmetic_failure_reason)
        -> !sec.core_error<"core::ArithmeticError">

    %err = "sec.result.err"(%error)
        : (!sec.core_error<"core::ArithmeticError">)
        -> !sec.result<T, !sec.core_error<"core::ArithmeticError">>

    func.return %err
```

---

# 40. `sec.fail.arithmetic` schema-v5 evolution

Schema-v5 compiler-generated form takes:

```text
reason: !sec.arithmetic_failure_reason
```

It remains:

```text
terminator
non-returning
panic-capable
non-speculatable
```

It no longer relies on a coarse static `category` attribute for the semantic
failure reason.

Optional diagnostic operator metadata may remain:

```text
sec.operator
```

The reason operand is authoritative.

---

# 41. Panic reason mapping

A later panic-endpoint lowering maps:

```text
Overflow
    -> panic.arithmetic-overflow

DivisionByZero
    -> panic.division-by-zero

InvalidShift
    -> panic.invalid-shift
```

Package 9 locks this mapping but does not choose:

```text
runtime symbol
trap instruction
target handler
PanicInfo physical layout
```

`sec.fail.arithmetic` remains the high-level panic-capable endpoint.

---

# 42. Why physical panic lowering is deferred

The panic rulebook allows the endpoint to be:

```text
inlined
compiler-emitted
application-provided
target-provided
target trap
```

and the illustrative root signature is not a finalized required source surface.

Package 9 therefore does not hard-code one runtime call.

This preserves Sec's no-mandatory-runtime architecture.

---

# 43. Checked guard verifier v5

Extend:

```bash
--sec-verify-checked-integer-guards
```

for schema v5.

Required checks:

```text
checked op has result + failed + reason
failed has exactly one canonical branch use
reason type is ArithmeticFailureReason
true edge passes exactly that reason to dedicated failure block
false edge does not pass/use failure reason
failure block receives exactly one reason block argument
ordinary failure block ends in sec.fail.arithmetic(reason)
fallible failure block converts that reason to ArithmeticError
checked result is unavailable on failure path
reason none cannot enter an ordinary/fallible failure endpoint
```

Schema-v4 regression mode may continue to validate old two-result fixtures.

---

# 44. Result-returning function type in MLIR

A Semantic IR function returning:

```text
Result[T, ArithmeticError]
```

lowers to:

```text
func.func ... -> !sec.result<T, !sec.core_error<"core::ArithmeticError">>
```

`func.return` returns exactly one Result SSA value.

Sec still has no multiple return values.

---

# 45. `Ok` return lowering

Source:

```sec
return Ok(value)
```

becomes:

```text
sec.result.ok
func.return
```

Source:

```sec
return Ok()
```

for:

```text
Result[void, E]
```

uses the zero-success-value form.

No special return terminator is needed.

---

# 46. `Err` return lowering

Source:

```sec
return Err(error)
```

becomes:

```text
sec.result.err
func.return
```

The error operand type must exactly match `E` for Package 9.

No automatic wrapping.

---

# 47. Package 9 Result boundary

Package 9 implements enough Result semantics for:

```text
function return type
Ok construction
Err construction
naked arithmetic try propagation
```

It does not yet implement:

```text
general Result-producing function call unwrapping
Result pattern matching
IsOk/IsErr lowering
local try handlers
fallback try expressions
try assignment
general match
Result physical representation
```

Do not broaden P9 into the whole error-handling system.

---

# 48. Local handler boundary

This Package 9 source form remains unsupported by the new Semantic IR path:

```sec
let value := try left + right {
    Err(error) => ...
}
```

The frontend may parse/type-check it according to existing rules.

Semantic IR lowering reports:

```text
semantic IR feature not implemented in Package 9:
local try handler
```

Do not silently convert it to naked propagation.

---

# 49. Error mapping boundary

This remains deferred:

```sec
fn Calculate(...) Result[int, InvoiceError] {
    let total := try left + right {
        Err(error) => return Err(InvoiceError.Calculation(error))
    }

    return Ok(total)
}
```

The compiler must not choose the wrapper variant.

Package 10 should implement local handler/mapping flow.

---

# 50. Panic-free binary rule

A program that uses only:

```text
fallible arithmetic through try
no ordinary panic-capable check
no other panic effect
```

must not require an arithmetic panic endpoint solely because the language
supports ordinary checked arithmetic elsewhere.

Package 9 must not introduce a mandatory panic runtime dependency.

---

# 51. Semantic IR verifier extensions

Verify:

```text
ArithmeticFailureReason value type
valid reason constants
checked result has correct reason result
ResultType has valid T/E
ResultOk operand matches T
ResultErr operand matches E
ArithmeticErrorFromReason output exact core ArithmeticError
ArithmeticFailureOp consumes non-none reason
fallible failure path returns Result with exact enclosing function type
```

Naked try propagation must end the failure path with `return`.

---

# 52. Sema tests

Required:

```text
try checked add -> ArithmeticError
try checked division -> ArithmeticError
try checked shift -> ArithmeticError
naked try accepted in Result[U, ArithmeticError]
naked try rejected in non-Result function
naked try rejected for Result[U, OtherError]
no implicit wrapper to InvoiceError
ordinary arithmetic retains panic effect
try arithmetic removes only arithmetic panic effect
operand-call panic effect remains
int128/int256/uint128/uint256 arithmetic try accepted normally
```

---

# 53. Semantic IR tests

## SIR01

Ordinary checked add emits:

```text
result
failed
reason
ordinary arithmetic failure block
```

## SIR02

Fallible checked add emits:

```text
result
failed
reason
ArithmeticErrorFromReason
ResultErr
return
```

## SIR03

Signed division dynamic reason distinguishes:

```text
zero -> DivisionByZero
min/-1 -> Overflow
```

## SIR04

Signed left shift distinguishes:

```text
bad count -> InvalidShift
representability failure -> Overflow
```

## SIR05

`Result[int128, ArithmeticError]` retains int128 as active type.

## SIR06

`Result[uint256, ArithmeticError]` retains uint256 as active type.

---

# 54. Dialect tests

Required:

```text
ArithmeticFailureReason type round-trip
reason constants round-trip
core error type round-trip
Result type round-trip
Result nested type identity
result.ok valid/invalid
result.err valid/invalid
arithmetic_error.from_reason valid
schema-v5 checked op result arity
schema-v5 fail arithmetic reason operand
schema-v4 regression fixtures remain parseable
wide Result types parse/verify
```

---

# 55. Guard verifier tests

Required:

```text
reason not passed to failure edge -> reject
wrong reason value passed -> reject
failure block wrong reason type -> reject
ordinary fail missing reason operand -> reject
fallible conversion from unrelated reason -> reject
success path consumes reason as error -> reject where structurally detectable
none reason constant routed to fail endpoint -> reject
division dynamic reason CFG accepted
signed-left dynamic reason CFG accepted
```

---

# 56. Package 8 correction tests

For post-P8 Arith lowering, verify reason generation.

Signed division:

```text
rhs == 0
    -> division_by_zero

else min/-1
    -> overflow

else
    -> none
```

Signed remainder:

same reason precedence.

Signed left shift:

```text
invalid count
    -> invalid_shift

else representability overflow
    -> overflow

else
    -> none
```

Add/sub/mul/neg:

```text
failed ? overflow : none
```

Unsigned shift:

```text
invalid ? invalid_shift : none
```

---

# 57. End-to-end ordinary arithmetic test

Pipeline:

```text
Sec source
    ↓
Sema
    ↓
Semantic IR
    ↓
schema-v5 Sec MLIR
    ↓
scalar core
    ↓
checked integer Arith lowering
```

Ordinary failure endpoint remains:

```text
sec.fail.arithmetic(reason)
```

Reason is correct.

---

# 58. End-to-end fallible arithmetic test

Source:

```sec
@noPanic
fn Add(left: int32, right: int32) Result[int32, ArithmeticError] {
    let total := try left + right
    return Ok(total)
}
```

Post-P8/P9 mixed MLIR must contain:

```text
standard Arith checked computation
dynamic reason value
cf.cond_br
ArithmeticError conversion
sec.result.err
sec.result.ok
func.return
```

and no arithmetic panic endpoint on the failure path.

---

# 59. Wide integer fallible tests

Required:

```text
int128 add try
int256 multiply try
uint128 subtract try
uint256 left shift try
```

All are active builtin operations.

No future/planned wording.

No host-width truncation.

---

# 60. No physical Result lowering

Package 9 does not choose:

```text
tag width
payload union layout
ABI aggregate layout
in-register return convention
stack return convention
LLVM struct representation
```

`!sec.result<T,E>` stays high-level.

That belongs to a later representation/ABI package.

---

# 61. No physical ArithmeticError lowering

Package 9 does not choose the enum's final integer width in MLIR lowering.

`!sec.core_error<"core::ArithmeticError">` remains high-level.

Later core/enum representation lowering may materialize a compact value.

---

# 62. No mandatory runtime

Package 9 fallible flow uses:

```text
ordinary SSA
branches
static error values
Result construction
```

It requires no:

```text
exception table
unwinder
heap allocation
runtime dispatcher
general Sec runtime library
```

---

# 63. Effect-analysis tests

Required:

```text
ordinary unproven add -> may arithmetic panic
try unproven add -> no arithmetic panic effect
try operand call with panic effect -> still may panic
Result return itself -> not a panic
ArithmeticError construction -> no panic
ResultErr construction -> no panic
```

---

# 64. Package 9 implementation layout

Recommended additions:

```text
internal/ir/semantic/
    result.go
    arithmetic_failure.go

mlir/include/sec/Dialect/Sec/
    SecTypes.td
    SecOps.td

mlir/lib/Dialect/Sec/
    SecTypes.cpp
    SecOps.cpp

mlir/test/Dialect/Sec/
    arithmetic-failure-reason.mlir
    result.mlir
    core-error.mlir
    arithmetic-error.mlir
    schema-v4-regression.mlir

mlir/test/Analysis/
    checked-integer-guards-v5.mlir

mlir/test/Conversion/SecIntegerToArith/
    failure-reasons.mlir
```

Use repository-consistent file organization if the actual implementation differs.

---

# 65. Acceptance criteria

Package 9 is complete only when:

```text
[ ] repository baseline is 152c772 or implementation report explains a newer sync
[ ] previous package regressions remain green
[ ] wide-builtin active-status invariant remains
[ ] core ArithmeticError minimum is defined
[ ] ArithmeticError has Overflow
[ ] ArithmeticError has DivisionByZero
[ ] ArithmeticError has InvalidShift
[ ] failure mapping is normative
[ ] Semantic IR ArithmeticFailureReason exists
[ ] checked integer ops expose result + failed + reason
[ ] failed/reason invariant is enforced
[ ] ordinary failure block receives reason
[ ] try failure block receives reason
[ ] ResultType exists in Semantic IR
[ ] ResultOkOp exists
[ ] ResultErrOp exists
[ ] ArithmeticErrorFromReasonOp exists
[ ] Sema exposes resolved try kind/error facts
[ ] naked arithmetic try propagation is implemented
[ ] propagation requires exact ArithmeticError in P9
[ ] local try handlers remain explicitly unsupported in new IR path
[ ] try does not catch arbitrary panic
[ ] effect analysis distinguishes ordinary from fallible arithmetic
[ ] @noPanic arithmetic try test passes
[ ] Sec MLIR schema is v5
[ ] !sec.arithmetic_failure_reason implemented
[ ] reason constant op implemented
[ ] !sec.core_error implemented
[ ] !sec.result implemented
[ ] sec.arithmetic_error.from_reason implemented
[ ] sec.result.ok implemented
[ ] sec.result.err implemented
[ ] sec.fail.arithmetic consumes reason in schema v5
[ ] checked guard verifier supports schema v5
[ ] Package 8 computes exact dynamic reason
[ ] zero divisor wins over signed min/-1 reason selection
[ ] invalid shift count wins over signed-left overflow reason selection
[ ] ordinary post-P8 failure keeps sec.fail.arithmetic
[ ] fallible post-P8 failure constructs Result Err
[ ] no physical Result layout is selected
[ ] no physical core-error layout is selected
[ ] no mandatory runtime is introduced
[ ] 128/256-bit fallible arithmetic tests pass
[ ] check-sec-mlir passes
[ ] go test ./... passes
[ ] legacy compiler paths remain operational
```

---

# 66. Required implementation report

Codex must report:

```text
1. repository HEAD implemented against
2. previous package status
3. core ArithmeticError rule change
4. files added
5. files modified
6. Sema resolved-try API
7. Semantic IR reason representation
8. Result Semantic IR representation
9. ordinary arithmetic failure CFG
10. fallible arithmetic failure CFG
11. error mapping implementation
12. effect-analysis changes
13. schema-v5 type additions
14. schema-v5 operation additions
15. checked-op v4/v5 compatibility strategy
16. checked-guard verifier changes
17. Package 8 reason-generation changes
18. division/remainder reason tests
19. shift reason tests
20. noPanic tests
21. wide integer fallible tests
22. CMake commands
23. exact LLVM/MLIR version
24. check-sec-mlir result
25. go test ./... result
26. end-to-end ordinary arithmetic results
27. end-to-end try arithmetic results
28. deviations
29. recommendations for Package 10
```

---

# 67. Package 10 boundary

Recommended Package 10:

```text
Local Try Handlers and Error Mapping
```

Scope:

```text
try expression handler blocks
Err(error) catch-all
Err(ArithmeticError.Variant) patterns
top-to-bottom handler selection
handler exhaustiveness
fallback value handlers
return/propagation handlers
explicit mapping to user error unions
Result success continuation
no automatic wrapper selection
Semantic IR handler CFG
Sec MLIR handler/match representation or direct verified CFG
```

Package 10 should not yet require:

```text
general enum match for unrelated types
all Result-returning call lowering
physical Result representation
LLVM dialect
```

After Package 10, the error pipeline can naturally expand to general Result
calls/match and then physical Result/ABI lowering.
