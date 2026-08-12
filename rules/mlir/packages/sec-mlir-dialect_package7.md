# Sec MLIR Program - Implementation Package 7

## Package status

Implementation package for the Sec compiler.

Package ID: `SEC-MLIR-P7`  
Package title: `Checked Integer Semantic Operations`  
Repository: `https://github.com/YoePro/sec`  
Repository branch: `main`  
Repository sync commit used for this package: `d48035c`  
Repository sync date: `2026-08-08`  
Semantic IR version: `1`  
Sec MLIR dialect schema before package: `3`  
Sec MLIR dialect schema after package: `4`  
Sec MLIR lowering specification after package: `3`

Package 7 implements the canonical checked integer operator layer from resolved
Sema facts through Semantic IR and into high-level Sec MLIR.

It intentionally stops before lowering those operations to MLIR `arith`.

The reason is semantic: ordinary Sec integer arithmetic is checked, division and
remainder have deterministic failure rules, and shifts have strict count rules.
Those semantics must be explicit before signed/unsigned integer types are
normalized to signless MLIR bitvectors.

---

# 1. Normative authority

Implementation follows:

```text
rules/operators.md
rules/runtime_checks.md
rules/types.md
rules/layout.md
    ↓
rules/semantic_ir.txt
    ↓
rules/sec_mlir.md
    ↓
rules/sec_mlir_dialect.md
    ↓
rules/sec_mlir_lowering.md
    ↓
implementation package
    ↓
implementation
```

Before implementation:

1. update `rules/sec_mlir_dialect.md` with the supplied
   `sec_mlir_dialect_package7.md`;
2. update `rules/sec_mlir_lowering.md` with the supplied
   `sec_mlir_lowering_package7.md`.

Do not change the source-language operator rules in this package.

---

# 2. Required wide-builtin status invariant

These are active Sec builtin types:

```text
int128
int256
uint128
uint256
decimal128
```

Package 7 must never describe the wide integer types as future, planned,
reserved, placeholder, or not yet active.

All integer operator implementation and tests in this package include the active
128-bit and 256-bit families.

`decimal128` is active as well, but decimal operators are outside Package 7.

---

# 3. Source semantics implemented by Package 7

Package 7 implements the integer subset of the canonical operator rulebook.

Included source operators:

```text
unary +
unary -
~

+
-
*
/
%

&
|
^

<<
>>

==
!=
<
<=
>
>=
```

Only builtin integer operands are in Package 7.

Excluded from Package 7:

```text
float operators
decimal operators
decimal128 operators
char/rune ordered comparison
string comparison
bool equality/logical operators
named numeric operator semantics
unit-bearing numeric operator semantics
compound assignment
try-selected arithmetic error flow
wrapping/saturating APIs
numeric conversion operators
matrix/vector/tensor operators
```

Those remain valid language areas where already defined, but are not part of
this implementation package.

---

# 4. Integer types supported

Package 7 supports these builtin integer semantic families:

```text
int
int8
int16
int32
int64
int128
int256

uint
uint8
uint16
uint32
uint64
uint128
uint256

byte
```

After Package 6 target resolution, the fixed lower-width forms are:

```text
si8
si16
si32
si64
si128
si256

ui8
ui16
ui32
ui64
ui128
ui256
```

Before Package 6 target resolution, high-level Sec MLIR may still contain:

```text
!sec.int
!sec.uint
```

`byte` uses unsigned 8-bit integer semantics.

Not treated as integer arithmetic types in Package 7:

```text
bool
char
rune
decimal
decimal128
float
float32
float64
named types
distinct types
unit-bearing types
bit[N] enum types
```

---

# 5. Core checked-arithmetic rule

Ordinary Sec integer arithmetic is checked.

For Package 7 checked operations, the semantic operation produces:

```text
result value
failure flag
```

where:

```text
failure flag type = bool / i1
false = operation succeeded
true  = operation would cause the language-defined arithmetic failure
```

The checked operation itself is semantically total.

It must not:

```text
invoke target undefined behavior
produce poison as its Sec-level failure behavior
trap implicitly
mask an invalid shift count
wrap an operation whose Sec semantics are checked
```

The failure is handled by explicit control flow after the operation.

---

# 6. Explicit failure-edge invariant

For every Package 7 checked integer operation generated from ordinary source:

```text
1. evaluate the left operand completely;
2. evaluate the right operand completely when present;
3. emit the checked semantic operation;
4. immediately test its failure flag;
5. branch to a dedicated arithmetic-failure block when true;
6. branch to a success continuation when false;
7. continue evaluating the enclosing expression only in the success block.
```

This is mandatory.

No later operand or source side effect may be evaluated between the checked
operation and its failure test.

This preserves:

```text
strict left-to-right evaluation
first-failure behavior
deterministic arithmetic failure
```

---

# 7. Ordinary versus `try`

Package 7 implements ordinary checked arithmetic failure control flow.

Ordinary failure lowers to:

```text
sec.fail.arithmetic
```

which is a non-returning high-level Sec MLIR terminator.

Package 7 does not yet implement:

```text
try arithmetic
ArithmeticError construction
Result propagation
local try handlers
error mapping
```

A source `try` expression containing a Package 7 integer operation must produce
`UnsupportedFeatureError` in the new Semantic IR pipeline until the Result/try
package is implemented.

Do not silently convert `try` arithmetic into panic behavior.

---

# 8. Compile-time invalid operations

Sema remains responsible for rejecting compile-time-proven invalid operations.

Examples:

```text
constant overflow
constant division by zero
constant invalid remainder divisor
constant invalid shift count
constant signed-left-shift overflow
negation of compile-time minimum signed value
```

Package 7 must not receive those expressions as valid runtime Semantic IR.

Package 7 is the runtime semantic representation after successful Sema.

---

# 9. Sema resolved-operator API

The Semantic IR builder must not reinterpret operator tokens.

Add a read-only resolved operator query.

Recommended model:

```go
type ResolvedOperatorKind string

type ResolvedOperator struct {
    Kind            ResolvedOperatorKind
    LeftType        Type
    RightType       *Type
    ResultType      Type
    RuntimeCheck    bool
    FailureBehavior OperatorFailureBehavior
}
```

Required integer kinds:

```text
integer-unary-plus
integer-negate-checked
integer-bit-not

integer-add-checked
integer-subtract-checked
integer-multiply-checked
integer-divide-checked
integer-remainder-checked

integer-bit-and
integer-bit-or
integer-bit-xor

integer-shift-left-unsigned-checked
integer-shift-left-signed-checked
integer-shift-right-unsigned-checked
integer-shift-right-signed-checked

integer-compare-eq
integer-compare-ne
integer-compare-lt
integer-compare-le
integer-compare-gt
integer-compare-ge
```

Recommended API:

```go
func (a *Analyzer) ResolvedOperatorOf(
    expr ast.Expression,
) (ResolvedOperator, bool)
```

The result must be recorded during successful Sema.

The query must not:

```text
perform operator resolution
perform type inference
perform overload resolution
mutate Analyzer
```

---

# 10. Operator classification is authoritative

The builder must not decide:

```text
signed versus unsigned shift
checked versus unchecked shift
signed versus unsigned comparison
integer versus float arithmetic
whether unary minus is valid
whether division can fail
whether a literal should be shaped
```

from token spelling.

Examples:

```text
">>" on signed integer
    -> integer-shift-right-signed-checked

">>" on unsigned integer
    -> integer-shift-right-unsigned-checked

"<" on signed integer
    -> integer-compare-lt with signed semantic operand

"<" on unsigned integer
    -> integer-compare-lt with unsigned semantic operand
```

These decisions come from Sema.

---

# 11. Semantic IR operation model

Extend:

```text
internal/ir/semantic
```

with canonical integer semantic operations.

Recommended closed operation families:

```go
type IntegerUnaryKind string
type IntegerCheckedBinaryKind string
type IntegerBitwiseKind string
type IntegerShiftKind string
type IntegerComparePredicate string
```

Recommended operation structures:

```go
type IntegerUnaryPlusOp struct
type IntegerNegCheckedOp struct
type IntegerBitNotOp struct
type IntegerCheckedBinaryOp struct
type IntegerBitwiseOp struct
type IntegerShiftCheckedOp struct
type IntegerCompareOp struct
type ArithmeticFailureOp struct
```

Exact Go struct names may differ.

Do not use a generic AST operator string as the semantic operation.

---

# 12. Semantic IR checked operation results

Checked operations return two Semantic IR values:

```text
result
failed
```

`failed` has canonical Semantic IR `bool` type.

Checked operations:

```text
integer negation
addition
subtraction
multiplication
division
remainder
all shifts
```

Non-failing Package 7 operations:

```text
unary plus
bitwise complement
bitwise AND
bitwise OR
bitwise XOR
integer comparison
```

---

# 13. Checked binary kind

Required kinds:

```text
add
subtract
multiply
divide
remainder
```

Input rules:

```text
left and right are the same resolved builtin integer type
result type equals operand type
```

Output:

```text
result: operand type
failed: bool
```

---

# 14. Addition failure meaning

For:

```text
add
```

`failed = true` exactly when the mathematical sum is not representable in the
resolved result type.

This applies to both:

```text
signed overflow
unsigned overflow
```

No wrapping is observable on failure.

---

# 15. Subtraction failure meaning

For:

```text
subtract
```

`failed = true` exactly when the mathematical difference is not representable.

Includes:

```text
signed overflow
unsigned underflow
```

---

# 16. Multiplication failure meaning

For:

```text
multiply
```

`failed = true` exactly when the mathematical product is not representable in
the result type.

---

# 17. Division failure meaning

Integer division truncates toward zero.

For signed operands:

```text
failed = true when divisor == 0
failed = true when left == minimum and right == -1
```

For unsigned operands:

```text
failed = true when divisor == 0
```

When `failed = false`, the result is the Sec truncation-toward-zero quotient.

---

# 18. Remainder failure meaning

Integer remainder uses the quotient from truncation-toward-zero division.

For signed operands:

```text
failed = true when divisor == 0
failed = true for the signed minimum / -1 overflow case
```

For unsigned operands:

```text
failed = true when divisor == 0
```

When successful:

```text
left == quotient * right + remainder
```

For signed operands, remainder has the sign of the left operand or is zero.

---

# 19. Checked negation

Unary negation is valid only for signed integer operands.

Output:

```text
result: same signed type
failed: bool
```

`failed = true` only for the minimum representable signed value.

Unsigned runtime negation must have been rejected by Sema.

---

# 20. Bitwise operations

Package 7 bitwise operations are total.

Kinds:

```text
not
and
or
xor
```

They use the fixed-width bit representation of the integer type.

Binary operands are the same resolved builtin integer type.

Result type equals operand type.

No bool operands.

---

# 21. Shift semantic kinds

Required Semantic IR shift kinds:

```text
shift-left-unsigned-checked
shift-left-signed-checked
shift-right-unsigned-checked
shift-right-signed-checked
```

The kind is selected by Sema from the left operand type.

The right operand is an integer shift-count value and need not have the same
integer type as the left operand.

Result type equals the left operand type.

All shift operations return:

```text
result
failed
```

---

# 22. Shift-count failure

For every shift:

```text
failed = true when count < 0
failed = true when count >= bit width of left operand
```

where a negative check is relevant only when the resolved count type can
represent negative values.

Sec never inherits target-specific masked shift counts.

---

# 23. Unsigned left shift

When the count is valid:

```text
unsigned left shift discards high bits
zeros enter from the right
```

This discard is defined bit behavior.

It is not arithmetic-overflow failure.

Therefore the only failure for unsigned left shift is invalid shift count.

---

# 24. Signed left shift

When the count is valid:

```text
failed = true if the mathematical shifted result is not representable
```

When successful, the exact representable shifted result is produced.

No silent wrap.

---

# 25. Right shifts

Signed right shift:

```text
arithmetic right shift
sign bit propagated
```

Unsigned right shift:

```text
logical right shift
zeros enter from the left
```

The only Package 7 failure is invalid shift count.

---

# 26. Integer comparisons

Semantic predicates:

```text
eq
ne
lt
le
gt
ge
```

Inputs:

```text
same resolved builtin integer type
```

Output:

```text
bool
```

Equality compares numeric value/bit-equivalent value as applicable.

Ordered predicates use signed order for signed integer types and unsigned order
for unsigned integer types.

No chained comparison exists at this layer; parser/Sema already rejected it.

---

# 27. Unary plus

Unary plus is represented explicitly in Semantic IR as an identity semantic
operation.

It:

```text
preserves type
preserves value
does not change signedness
does not fail
```

The Sec MLIR dialect may later fold it to its operand.

Do not omit it before Semantic IR if doing so would violate operator provenance
or tooling expectations.

---

# 28. Arithmetic failure terminator in Semantic IR

Add a Semantic IR terminator representing ordinary deterministic arithmetic
failure.

Recommended:

```go
type ArithmeticFailureCategory string

const (
    ArithmeticFailureOverflow  ArithmeticFailureCategory = "overflow"
    ArithmeticFailureDivision  ArithmeticFailureCategory = "division"
    ArithmeticFailureRemainder ArithmeticFailureCategory = "remainder"
    ArithmeticFailureShift     ArithmeticFailureCategory = "shift"
)
```

Recommended operation:

```go
type ArithmeticFailureOp struct {
    Category ArithmeticFailureCategory
    Operator string
    Location Location
}
```

It is a terminator.

It has no successor.

It is not a process exit decision.

It is not yet a runtime call.

It represents the language-level non-returning ordinary arithmetic failure path.

---

# 29. Canonical Semantic IR block shape

For:

```sec
return left + right
```

canonical conceptual Semantic IR:

```text
^0:
    %sum, %failed = int.binary.checked add %left, %right
    cond_br %failed, ^1, ^2

^1:
    arithmetic.fail overflow "+"

^2:
    return %sum
```

For nested checked arithmetic:

```sec
return a + b * c
```

canonical evaluation order:

```text
evaluate a
evaluate b
evaluate c

%product, %mulFailed = checked multiply
cond_br %mulFailed, mulFailure, afterMultiply

afterMultiply:
    %sum, %addFailed = checked add
    cond_br %addFailed, addFailure, afterAdd

afterAdd:
    return %sum
```

No addition occurs before multiplication has succeeded.

---

# 30. Dedicated failure blocks

Package 7 builder creates one dedicated arithmetic failure block for each
runtime checked operation.

Do not merge failure blocks during builder construction.

Reason:

```text
simple provenance
simple verifier
precise source location
precise operator category
first-failure behavior
```

A later canonicalization pass may merge proven-equivalent failure endpoints only
when source attribution and observable behavior remain correct.

---

# 31. Semantic IR verifier extensions

Verify:

```text
checked operation integer operand types are supported
binary operand types match
checked result type matches operand/result semantic type
failure result is bool
negation operand is signed
bitwise operand is integer
shift left operand signedness matches shift semantic kind
shift count is integer
comparison result is bool
comparison operand types match
ArithmeticFailureOp is a terminator
ArithmeticFailureOp category is valid
```

Also verify the canonical checked-operation guard shape:

```text
checked op is the last non-terminator in its block
block terminator is ConditionalBranchOp
branch condition is exactly the checked op failure ValueID
true successor is the dedicated arithmetic-failure block
false successor is the success continuation
failure block contains only ArithmeticFailureOp
failure block has no successor
checked result has no use in the failure block
```

This cross-operation invariant may be implemented by the module verifier.

---

# 32. Sema and builder tests

Required Sema tests:

```text
signed >> resolves signed-right kind
unsigned >> resolves unsigned-right kind
signed << resolves signed-left-checked kind
unsigned << resolves unsigned-left-checked kind
signed comparison retains signed operand semantics
unsigned comparison retains unsigned operand semantics
unary minus on unsigned rejected
bitwise bool rejected
all active 128/256-bit integer types resolve normally
```

Required builder tests:

```text
left-to-right nested arithmetic
checked op creates dedicated failure block
failure flag immediately controls branch
no later operand evaluation occurs before guard
division gets division failure category
remainder gets remainder failure category
shift gets shift failure category
add/sub/mul/neg get overflow category
```

---

# 33. Sec MLIR dialect schema version 4

Compiler-generated high-level Sec MLIR now uses:

```mlir
sec.dialect_version = 4 : i32
```

Schema versions 1-3 remain parseable for regression tests.

Schema 4 adds integer operator operations and arithmetic-failure termination.

No scalar type is removed.

---

# 34. New Sec MLIR operations

Schema version 4 adds:

```text
sec.int.unary_plus
sec.int.neg_checked
sec.int.binary_checked
sec.int.bit_not
sec.int.bitwise
sec.int.shift_checked
sec.int.cmp

sec.fail.arithmetic
```

Use TableGen/ODS.

Do not define one operation per source token where a closed enum attribute gives
the same semantic clarity.

---

# 35. Integer type category accepted by schema 4

High-level integer operator operands may be:

```text
!sec.int
!sec.uint

si8
si16
si32
si64
si128
si256

ui8
ui16
ui32
ui64
ui128
ui256
```

`ui8` also represents `byte`.

Excluded:

```text
i1
!sec.char
!sec.rune
!sec.decimal
!sec.decimal128
!sec.float
f32
f64
!sec.named
!sec.distinct
```

Named/distinct integer operator support is deferred.

---

# 36. `sec.int.unary_plus`

Operands:

```text
value: integer semantic type T
```

Results:

```text
result: T
```

Verifier:

```text
T is schema-v4 builtin integer semantic type
result type equals operand type
```

No failure.

No memory effects.

May be canonicalized to operand later.

---

# 37. `sec.int.neg_checked`

Operand:

```text
value: signed integer semantic type T
```

Results:

```text
result: T
failed: i1
```

Verifier:

```text
T is signed
result type equals T
failed type is i1
```

Semantic guarantee:

```text
failed true exactly for minimum signed value
operation itself is total
```

---

# 38. `sec.int.binary_checked`

Required enum attribute:

```text
kind
```

Allowed:

```text
add
subtract
multiply
divide
remainder
```

Operands:

```text
left: T
right: T
```

Results:

```text
result: T
failed: i1
```

Verifier:

```text
T is supported builtin integer semantic type
operand types equal
result type equals operands
failed is i1
```

Operation semantics are those defined in this package and `operators.md`.

---

# 39. `sec.int.bit_not`

Operand/result:

```text
T -> T
```

T is supported builtin integer semantic type.

No failure.

---

# 40. `sec.int.bitwise`

Required enum:

```text
and
or
xor
```

Operands:

```text
T, T
```

Result:

```text
T
```

No failure.

---

# 41. `sec.int.shift_checked`

Required enum:

```text
left_unsigned
left_signed
right_unsigned
right_signed
```

Operands:

```text
value: T
count: C
```

where:

```text
T is supported builtin integer semantic type
C is supported builtin integer semantic type
```

Results:

```text
result: T
failed: i1
```

Verifier:

```text
left_unsigned/right_unsigned require unsigned T
left_signed/right_signed require signed T
result type equals T
failed is i1
```

Count type may differ from T.

---

# 42. `sec.int.cmp`

Required enum:

```text
eq
ne
lt
le
gt
ge
```

Operands:

```text
left: T
right: T
```

Result:

```text
i1
```

Verifier:

```text
operand types equal
T is supported builtin integer semantic type
result is i1
```

Signed versus unsigned ordering follows T.

---

# 43. Checked-operation totality

All schema-v4 checked integer operations are total high-level Sec operations.

This is a normative dialect property.

For any bit pattern representable by their operand types:

```text
the operation itself is defined
the operation returns a result bit pattern
the operation returns failed=true when Sec would fail
```

When `failed=true`, the result value is semantically unavailable to source code.

The canonical guard invariant prevents it from being consumed on the failure
path.

The MLIR lowering must never implement the high-level operation using a target
operation that has already invoked UB/poison before `failed` can be computed.

---

# 44. `sec.fail.arithmetic`

No operands.

No results.

No successors.

Terminator.

Required enum attribute:

```text
category
```

Allowed:

```text
overflow
division
remainder
shift
```

Required attribute:

```text
sec.operator
```

String value examples:

```text
"+"
"-"
"*"
"/"
"%"
"<<"
">>"
```

The operation represents the ordinary deterministic non-returning arithmetic
failure endpoint.

It does not specify:

```text
runtime function name
trap instruction
process termination mechanism
exception
unwinder
```

Those are lower-layer decisions.

---

# 45. MLIR canonical checked guard shape

Compiler-generated schema-v4 checked arithmetic must have:

```mlir
%result, %failed = "sec.int.binary_checked"(...)
    {kind = ...} : (...) -> (T, i1)

cf.cond_br %failed, ^failure, ^success
```

The true successor is the failure path.

The false successor is the success path.

Failure block canonical form:

```mlir
^failure:
    "sec.fail.arithmetic"() {
        category = ...,
        sec.operator = "..."
    } : () -> ()
```

No operation may consume `%result` before the guard.

---

# 46. Checked integer guard verifier pass

Add:

```bash
--sec-verify-checked-integer-guards
```

Recommended implementation:

```text
function-level verification pass
```

It does not modify IR.

It verifies compiler-generated schema-v4 guard invariants.

Required checks:

```text
every checked integer op failure result has exactly one use
that use is the immediately following cf.cond_br condition
checked op is the final non-terminator in its block
cf.cond_br true successor is dedicated arithmetic failure block
cf.cond_br false successor is success continuation
failure block contains one sec.fail.arithmetic terminator
failure block has no block arguments
failure category matches checked operation family
sec.operator matches semantic operation
checked result is not used in failure block
```

The pass may use dominance analysis for stronger result-use validation.

---

# 47. Compiler verification pipeline

Schema-v4 `emit-sec-mlir` verification must run:

```text
normal MLIR verification
sec-verify-checked-integer-guards
```

Output is published only after both succeed.

Do not use `--allow-unregistered-dialect`.

---

# 48. Package 6 scalar-resolution compatibility

Extend:

```text
--sec-resolve-scalar-layout
```

so schema-v4 integer operations survive type resolution.

Required:

```text
!sec.int operands/results -> si32/si64
!sec.uint operands/results -> ui32/ui64
shift count types resolve independently
operation kind unchanged
failure i1 unchanged
comparison result i1 unchanged
failure blocks unchanged
```

Package 6 must not lower the new integer operations.

---

# 49. Package 5 compatibility

`--sec-lower-trivial-core` must leave schema-v4 integer semantic operations
untouched.

It may continue to lower:

```text
bool constants
eligible storage
direct calls
```

No Package 5 pattern may replace integer checked operations with `arith` yet.

---

# 50. Scalar-core pipeline compatibility

After:

```bash
--sec-lower-scalar-core
```

schema-v4 integer semantic operations remain, but:

```text
target-sized !sec.int/!sec.uint have resolved width
plain integer constants may already be arith.constant
eligible storage may already be memref
direct calls may already be func.call
```

Signed/unsigned integer operator operand types remain `siN`/`uiN`.

This is expected.

---

# 51. No signless normalization in Package 7

Do not convert:

```text
siN -> iN
uiN -> iN
```

in Package 7.

Do not lower checked integer operations to:

```text
arith.addi
arith.subi
arith.muli
arith.divsi
arith.divui
arith.remsi
arith.remui
arith.shli
arith.shrsi
arith.shrui
arith.cmpi
```

yet.

That belongs to Package 8.

Package 7's purpose is to make the semantic distinctions explicit enough that
Package 8 can perform that conversion safely.

---

# 52. No use of Arith overflow flags as Sec checks

Package 7 must not treat:

```text
overflow<nsw>
overflow<nuw>
```

as implementation of Sec checked arithmetic.

Those flags make violating operations poison in MLIR.

Sec requires deterministic failure.

They may later be used only after proof that overflow cannot occur or in a
context whose semantics explicitly permit poison.

---

# 53. No unsafe division or shift lowering

Package 7 must not emit raw standard operations that are undefined/poison for
inputs Sec is required to check first.

Examples:

```text
arith.divsi before checking divisor and minimum/-1
arith.divui before checking divisor
arith.shli before checking count range
arith.shrsi before checking count range
arith.shrui before checking count range
```

The high-level Sec checked op exists specifically to prevent premature unsafe
lowering.

---

# 54. Required Semantic IR source tests

## S01 - signed addition

```sec
fn Add(a: int32, b: int32) int32 {
    return a + b
}
```

Expected:

```text
checked add
failure flag
overflow failure block
success continuation
```

## S02 - unsigned subtraction

Must retain checked underflow behavior.

## S03 - int128 multiplication

Must be represented exactly like smaller checked integer multiplication.

No future/planned classification.

## S04 - uint256 addition

Must be represented normally.

## S05 - signed division

Must use checked division semantic op.

## S06 - unsigned division

Same operation family with unsigned operand semantics retained.

## S07 - remainder

Checked remainder + dedicated remainder failure category.

## S08 - signed unary negation

Checked negation.

## S09 - bitwise operators

No failure blocks.

## S10 - signed right shift

Semantic kind signed-right.

## S11 - unsigned right shift

Semantic kind unsigned-right.

## S12 - signed left shift

Checked count + checked representability semantic kind.

## S13 - unsigned left shift

Checked count but truncating high-bit semantics.

## S14 - integer comparison

All six predicates.

## S15 - nested arithmetic order

```sec
return First() + Second() * Third()
```

Required observable sequence:

```text
First call
Second call
Third call
multiply
multiply guard
add
add guard
```

---

# 55. Required unsupported source tests

## U01 - try arithmetic

Expected Package 7 Semantic IR unsupported error.

## U02 - float arithmetic

Not part of Package 7.

## U03 - decimal arithmetic

Not part of Package 7.

## U04 - named integer arithmetic

Not part of Package 7 new pipeline.

## U05 - unit-bearing arithmetic

Not part of Package 7.

## U06 - compound assignment

Still deferred.

These are pipeline implementation boundaries, not language errors.

---

# 56. Required dialect tests

## D01

All new operation kinds parse/print.

## D02

`sec.int.binary_checked` rejects mismatched operand types.

## D03

Checked failure result must be i1.

## D04

Signed negation rejects unsigned operand.

## D05

Signed shift kind rejects unsigned left operand.

## D06

Unsigned shift kind rejects signed left operand.

## D07

Shift count accepts different integer width.

## D08

Comparison returns i1.

## D09

Named integer type rejected by schema-v4 builtin-integer ops.

## D10

`int128` and `uint256` operation examples verify.

## D11

`sec.fail.arithmetic` rejects unknown category.

## D12

`sec.fail.arithmetic` is a terminator.

## D13

Schema versions 1-3 regression tests remain parseable.

---

# 57. Required checked-guard verifier tests

## G01 - canonical add guard accepted

## G02 - failure flag unused rejected

## G03 - failure flag used twice rejected

## G04 - non-branch failure-flag use rejected

## G05 - reversed true/false targets rejected

## G06 - operation between checked op and branch rejected

## G07 - failure block with extra operation rejected

## G08 - failure block without fail terminator rejected

## G09 - wrong failure category rejected

## G10 - wrong operator provenance rejected

## G11 - checked result used in failure block rejected

## G12 - nested checked operations in separate canonical blocks accepted

---

# 58. Required Package 6 compatibility tests

Target 32:

```text
!sec.int checked add -> si32 checked add
!sec.uint checked compare -> ui32 checked compare
```

Target 64:

```text
!sec.int checked add -> si64 checked add
!sec.uint checked compare -> ui64 checked compare
```

All active wide integer types remain unchanged:

```text
si128
si256
ui128
ui256
```

No width loss.

---

# 59. Required end-to-end tests

Pipeline:

```text
Sec source
    ↓
Sema
    ↓
Semantic IR v1
    ↓
Sec MLIR schema v4
    ↓
sec-verify-checked-integer-guards
    ↓
sec-lower-scalar-core
    ↓
sec-verify-checked-integer-guards
```

Representative cases:

```text
int32 add
uint32 subtract underflow-capable
int64 divide
uint64 remainder
int128 multiply
int256 signed shift
uint128 unsigned shift
uint256 compare
target-sized int on 32-bit target
target-sized int on 64-bit target
nested arithmetic with calls
if condition using integer comparison
```

Generated MLIR must require no hand editing.

---

# 60. No production-backend migration

Package 7 does not replace:

```text
emit-mlir
emit-llvm
build
```

The new Semantic IR/Sec MLIR pipeline remains independently testable.

Legacy backend tests remain green.

---

# 61. MLIR implementation layout

Extend:

```text
mlir/include/sec/Dialect/Sec/SecOps.td
mlir/lib/Dialect/Sec/SecOps.cpp
```

Add verifier pass infrastructure, recommended:

```text
mlir/include/sec/Analysis/
    CMakeLists.txt
    Passes.h
    Passes.td

mlir/lib/Analysis/
    CMakeLists.txt
    VerifyCheckedIntegerGuards.cpp

mlir/test/Dialect/Sec/
    integer-ops-roundtrip.mlir
    integer-ops-invalid.mlir
    wide-integer-ops.mlir

mlir/test/Analysis/
    checked-integer-guards.mlir
    checked-integer-guards-invalid.mlir
```

A different repository-consistent analysis path is acceptable.

Do not hide the verifier logic inside unrelated lowering code.

---

# 62. Operation effects

Integer semantic operations:

```text
no memory effects
```

They compute semantic values/failure flags only.

`sec.fail.arithmetic`:

```text
non-returning observable failure
not speculatable
not removable as a pure operation
```

Do not mark the failure terminator as `NoMemoryEffect` merely to simplify
optimization.

---

# 63. Canonicalization boundary

Package 7 may add only obviously semantics-preserving folds.

Allowed examples:

```text
unary plus -> operand
bitwise x & x -> x
bitwise x | x -> x
bitwise x ^ x -> zero only when type-safe constant construction exists
comparison x == x only when integer SSA identity makes result unconditionally true
```

Do not add checked arithmetic folds unless they also preserve the failure flag.

Do not make canonicalization a package acceptance dependency.

---

# 64. Error policy

Invalid source:

```text
Sema diagnostic
```

Valid but Package-7-unsupported source:

```text
UnsupportedFeatureError
```

Malformed Semantic IR:

```text
Semantic IR verifier/internal compiler error
```

Malformed Sec MLIR operation:

```text
MLIR verifier error
```

Invalid checked-op guard structure:

```text
sec-verify-checked-integer-guards failure
```

Do not turn compiler-internal verification failures into source diagnostics.

---

# 65. Architecture rules

Non-negotiable:

```text
Operator meaning comes from Sema, not token spelling.

Every checked integer operation is explicit in Semantic IR.

Every runtime checked operation has an explicit failure flag.

Every ordinary runtime checked operation immediately branches on that flag.

Failure is deterministic.

No checked operation invokes target UB/poison as Sec failure semantics.

Evaluation remains left to right.

First failure stops later expression evaluation.

Unsigned left shift truncates high bits but still validates count.

Signed left shift validates count and representability.

Signed right shift is arithmetic.

Unsigned right shift is logical.

Division truncates toward zero.

Remainder follows truncation-based quotient semantics.

128/256-bit integer families are ordinary active builtins.

No signedness erasure occurs yet.

No Arith lowering occurs yet.

No float/decimal semantics are mixed into this package.

No LLVM dialect is emitted.
```

---

# 66. Acceptance criteria

Package 7 is complete only when:

```text
[ ] previous package regressions remain green
[ ] wide-builtin cleanup invariant is preserved
[ ] rules/sec_mlir_dialect.md updated to schema v4
[ ] rules/sec_mlir_lowering.md updated to lowering spec v3
[ ] Sema exposes read-only resolved operator metadata
[ ] builder never re-resolves integer operators
[ ] Semantic IR integer unary plus exists
[ ] Semantic IR checked negation exists
[ ] Semantic IR checked binary arithmetic exists
[ ] Semantic IR bitwise operations exist
[ ] Semantic IR checked shift operations exist
[ ] Semantic IR integer comparisons exist
[ ] Semantic IR arithmetic-failure terminator exists
[ ] every checked op returns result + bool failure flag
[ ] checked-operation guard CFG is canonical
[ ] left-to-right nested arithmetic is preserved
[ ] first-failure behavior is preserved
[ ] all active widths 8/16/32/64/128/256 are tested
[ ] !sec.int/!sec.uint high-level operands are supported before P6 resolution
[ ] dialect schema version is 4
[ ] sec.int.unary_plus implemented
[ ] sec.int.neg_checked implemented
[ ] sec.int.binary_checked implemented
[ ] sec.int.bit_not implemented
[ ] sec.int.bitwise implemented
[ ] sec.int.shift_checked implemented
[ ] sec.int.cmp implemented
[ ] sec.fail.arithmetic implemented
[ ] checked ops are semantically total
[ ] sec.fail.arithmetic is non-returning
[ ] --sec-verify-checked-integer-guards registered
[ ] malformed guard CFG is rejected
[ ] Package 6 scalar-resolution pass preserves new ops and resolves their types
[ ] Package 5 trivial-core pass leaves new integer ops untouched
[ ] scalar-core pipeline leaves checked integer semantics explicit
[ ] no siN/uiN -> iN conversion occurs
[ ] no arith integer operation lowering occurs
[ ] no LLVM dialect operation is emitted
[ ] check-sec-mlir passes
[ ] go test ./... passes
[ ] legacy compiler paths remain operational
```

---

# 67. Required implementation report

Codex must report:

```text
1. repository HEAD implemented against
2. previous package status
3. files added
4. files modified
5. Sema resolved-operator API
6. Semantic IR integer operation API
7. checked-operation block construction algorithm
8. failure categories
9. schema-v4 operation definitions
10. ODS/custom verifiers
11. checked-guard verifier implementation
12. Package 6 compatibility changes
13. active 128/256-bit test coverage
14. source evaluation-order tests
15. dialect test commands/results
16. check-sec-mlir result
17. go test ./... result
18. end-to-end source -> schema-v4 MLIR results
19. unsupported source forms encountered
20. deviations
21. recommendations for Package 8
```

---

# 68. Package 8 boundary

Package 8 should be:

```text
Checked Integer Arith Lowering
```

It should consume the schema-v4 semantic integer operations and lower them to
standard signless MLIR integer operations while preserving explicit Sec failure
flags.

Recommended Package 8 scope:

```text
siN/uiN -> signless iN normalization
preserve original signedness in semantic operation selection/metadata
integer constants to signless arith.constant
unary plus elimination
bitwise lowering
integer comparisons:
    signed -> signed arith.cmpi predicates
    unsigned -> unsigned arith.cmpi predicates

checked addition/subtraction/multiplication:
    compute wrapped candidate safely
    compute failure without poison/UB
    support widths 8/16/32/64/128/256

checked division/remainder:
    never execute unsafe div/rem for zero or signed min/-1
    produce failure flag deterministically

checked shifts:
    validate count before any target shift
    unsigned left truncation
    signed-left representability
    signed arithmetic right shift
    unsigned logical right shift

preserve existing cf.cond_br failure edges
leave sec.fail.arithmetic explicit
no poison-based overflow semantics
no LLVM dialect
```

Package 8 should still defer:

```text
try/ArithmeticError
panic endpoint lowering
float operators
float literal rounding
decimal operators
named/unit numeric operators
numeric casts
foreign ABI
ownership
Result/try general lowering
aggregates
allocation
register/MMIO
concurrency
LLVM dialect
```

The critical Package 8 invariant is:

```text
schema-v4 checked integer operation
    ↓
total standard MLIR computation of (result, failed)
    ↓
existing explicit Sec failure edge

with no target UB, poison, masked shift count, or silent wrap escaping as Sec
behavior.
```

---

# 69. Upstream MLIR references

Implement against the exact project-selected MLIR version.

Relevant upstream references:

```text
https://mlir.llvm.org/docs/Dialects/ArithOps/
https://mlir.llvm.org/docs/DialectConversion/
```

Important Package 8 preparation facts from current MLIR:

```text
arith integer arithmetic uses signless bitvector integer operands
signed/unsigned division use different operations
signed/unsigned ordered compare use different predicates
signed/unsigned right shift use different operations
shift counts outside width may produce poison
integer overflow flags produce poison when their assumptions are violated
```

Therefore Package 7 must preserve semantic signedness and explicit failure
control before any such lowering.
