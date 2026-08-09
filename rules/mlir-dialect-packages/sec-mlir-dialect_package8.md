# Sec MLIR Program - Implementation Package 8

## Package status

Implementation package for the Sec compiler.

Package ID: `SEC-MLIR-P8`\
Package title: `Checked Integer Arith Lowering`\
Repository: `https://github.com/YoePro/sec`\
Repository branch: `main`\
Repository sync commit used for this package: `d48035c`\
Repository sync date: `2026-08-09`\
Required Semantic IR version: `1`\
Required Sec MLIR dialect schema: `4`\
Sec MLIR dialect schema after package: `4`\
Sec MLIR lowering specification after package: `4`

Package 8 lowers schema-v4 builtin integer semantic operations to standard MLIR
`arith` operations while preserving Sec checked-arithmetic behavior.

The package performs two related representation changes:

```text
schema-v4 checked integer semantic operations
    -> total signless Arith computation of (result, failed)

plain builtin signed/unsigned MLIR integer representation
    -> signless MLIR integer representation
```

The existing explicit Sec failure CFG remains in place.

`sec.fail.arithmetic` remains a Sec operation.

Package 8 does not lower to LLVM.

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

Before implementation, update:

```text
rules/sec_mlir_lowering.md
```

with the supplied:

```text
sec_mlir_lowering_package8.md
```

No `rules/sec_mlir_dialect.md` update is required because Package 8 adds no new
Sec dialect type or operation.

---

# 2. Repository and upstream facts used by this package

The Sec operator rules require:

```text
checked integer addition
checked integer subtraction
checked integer multiplication
checked integer division overflow
checked integer remainder overflow where applicable
checked unary negation
checked signed left shift
deterministic runtime arithmetic failure
strict shift-count validation
signed arithmetic right shift
unsigned logical right shift
unsigned left-shift high-bit truncation
```

The MLIR Arith dialect has materially different low-level behavior:

```text
arith.addi/subi/muli without overflow flags are modulo bitvector operations
arith overflow flags produce poison when their assumptions are violated
arith.divsi has undefined behavior on zero divisor and signed min / -1
arith.divui has undefined behavior on zero divisor
arith.remsi/remui have undefined behavior on zero divisor
arith.shli/shrsi/shrui may produce poison when count >= value width
Arith integer operators use signless integer types
signed versus unsigned comparison is encoded in arith.cmpi predicate
```

Package 8 must bridge those models explicitly.

---

# 3. Wide builtin invariant

These integer types are active Sec builtins and are fully in scope:

```text
int128
int256
uint128
uint256
```

All Package 8 algorithms must work at widths:

```text
8
16
32
64
128
256
```

and at target-resolved `int`/`uint` width:

```text
32 or 64
```

No implementation may narrow through host `int64`, host `uint64`, or native C++
integer arithmetic.

Use MLIR/APInt arbitrary-width facilities.

`decimal128` remains an active Sec builtin but is not an integer and is outside
Package 8 arithmetic lowering.

---

# 4. Package goal

After Package 8:

1. schema-v4 integer semantic ops no longer remain after the integer-lowering
   pass;
2. their results are computed by standard signless Arith operations;
3. every checked result still has an explicit deterministic failure flag;
4. the existing `cf.cond_br` failure edge remains;
5. no lower operation executes undefined behavior for an input Sec must handle;
6. no lower operation produces poison as the implementation of a Sec failure;
7. signed and unsigned semantics select different lower predicates/operations
   where required;
8. ordinary integer representation inside the lowered core is signless `iN`;
9. source scalar provenance required by later ABI/FFI lowering is preserved at
   semantic boundaries before signedness type information is erased;
10. direct calls and ordinary function signatures use compatible signless core
    types;
11. foreign calls remain `sec.call.foreign`;
12. foreign function parameter/result provenance remains sufficient for later
    ABI lowering;
13. integer storage created by earlier packages uses signless element types;
14. named/distinct type identity is not erased;
15. float and decimal lowering remain untouched;
16. `sec.fail.arithmetic` remains explicit;
17. no LLVM dialect operation is created.

---

# 5. Package boundary

## 5.1 In scope

Implement:

```text
lowering-spec version 4
scalar provenance metadata before signless erasure
Package 6 provenance correction
--sec-lower-checked-integers
SecIntegerToArith conversion library
signless integer TypeConverter
plain siN/uiN -> iN conversion
function signature conversion
block argument conversion
func.call conversion
sec.call.direct compatibility
sec.call.foreign signature conversion without call lowering
rank-zero Sec-origin memref integer element conversion
arith.constant integer type/attribute conversion
sec.int.unary_plus lowering
sec.int.neg_checked lowering
sec.int.binary_checked add lowering
sec.int.binary_checked subtract lowering
sec.int.binary_checked multiply lowering
sec.int.binary_checked divide lowering
sec.int.binary_checked remainder lowering
sec.int.bit_not lowering
sec.int.bitwise lowering
sec.int.shift_checked lowering
sec.int.cmp lowering
safe shift-count normalization
safe division/remainder operand substitution
total checked result/failure computation
preservation of failure CFG
pre-lowering checked-guard verification
post-lowering absence verification
idempotence
8/16/32/64/128/256 tests
32/64-bit target-sized integration tests
full regression
```

## 5.2 Explicitly out of scope

Do not implement:

```text
sec.fail.arithmetic lowering
panic/runtime failure endpoint lowering
try arithmetic
ArithmeticError
Result propagation

float arithmetic
float literal lowering
decimal arithmetic
decimal128 arithmetic

numeric source conversions/casts
checked narrowing
float-to-integer conversion

named integer arithmetic
distinct integer arithmetic
unit-bearing integer arithmetic
nominal type erasure

foreign ABI lowering
calling-convention lowering
link-name lowering
varargs lowering

ownership
copy
move
destruction
borrow
references
RawPtr
cleanup
defer

aggregates
allocation
arena
register/MMIO
volatile
atomics
concurrency

LLVM dialect
LLVM IR
object generation
production backend migration
```

---

# 6. Mandatory scalar-provenance correction

Package 6 resolves high-level semantic scalar types such as:

```text
!sec.int
!sec.uint
!sec.char
!sec.rune
```

to concrete builtin integer representation.

Package 8 then normalizes plain signed/unsigned builtin integers to signless
`iN`.

Before this information is erased from types, compiler-generated boundaries must
retain the original semantic scalar kind.

This is required especially for future:

```text
FFI
ABI argument extension
external return classification
diagnostics
debug information
```

---

# 7. `sec.scalar_kind` metadata

Define lowering metadata:

```text
sec.scalar_kind
```

Allowed Package 8 values:

```text
bool
byte
char
rune

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

float
float32
float64

decimal
decimal128

string
other
```

This metadata records semantic origin.

It does not determine source semantics after Sema.

It does not replace nominal type identity.

Named/distinct values keep their nominal Sec type and are not normalized by this
package.

---

# 8. Function argument/result provenance

Before Package 8 signless conversion, Package 6/high-level emission must attach
`sec.scalar_kind` to plain builtin scalar function arguments and results.

Use normal `func.func` argument/result attributes.

Conceptually:

```mlir
func.func @sec_fn_0(
    %arg0: si32 {sec.scalar_kind = "int32"},
    %arg1: ui32 {sec.scalar_kind = "uint32"}
) -> (ui64 {sec.scalar_kind = "uint64"})
```

Use the exact syntax supported by the selected MLIR release.

After Package 8 type conversion:

```text
si32 -> i32
ui32 -> i32
ui64 -> i64
```

but the argument/result metadata remains.

For extern functions this metadata is mandatory for every plain integer
parameter/result that is normalized.

---

# 9. Storage provenance

`sec.storage.declare` and its later `memref.alloca` replacement must preserve:

```text
sec.scalar_kind
```

when the storage contains a plain builtin scalar.

Existing provenance remains:

```text
sec.storage_id
sec.source_name
sec.storage_class
sec.mutable
```

Example:

```text
uint32 local storage
    -> memref<i32>
    + sec.scalar_kind = "uint32"
```

---

# 10. Signless integer conversion domain

Package 8 converts plain builtin integer types:

```text
si8   -> i8
si16  -> i16
si32  -> i32
si64  -> i64
si128 -> i128
si256 -> i256

ui8   -> i8
ui16  -> i16
ui32  -> i32
ui64  -> i64
ui128 -> i128
ui256 -> i256
```

Already signless integer types remain unchanged.

Do not convert through the host integer type.

---

# 11. Types not normalized by Package 8

Do not recursively erase or normalize:

```text
!sec.named<...>
!sec.distinct<...>
!sec.string
!sec.decimal
!sec.decimal128
!sec.never
```

If a named/distinct base is still `siN`/`uiN`, leave the wrapper and base
unchanged in Package 8.

Nominal semantic identity is a separate lowering obligation.

---

# 12. Pass preconditions

Canonical pass:

```bash
--sec-lower-checked-integers
```

Preconditions:

```text
normal MLIR verification succeeds
Package 7 checked-guard verification succeeds
target-sized !sec.int/!sec.uint have already been resolved by Package 6
no schema-v4 checked integer operation uses !sec.int or !sec.uint
no schema-v4 checked integer operation uses named/distinct operands
```

If unresolved target-sized integer types remain, fail clearly.

---

# 13. Pass implementation

Use MLIR Dialect Conversion.

Recommended layout:

```text
mlir/include/sec/Conversion/SecIntegerToArith/
    CMakeLists.txt
    Passes.h
    Passes.td

mlir/lib/Conversion/SecIntegerToArith/
    CMakeLists.txt
    SecIntegerToArith.cpp
    CheckedArithmetic.cpp
    CheckedDivision.cpp
    CheckedShift.cpp
    IntegerTypeConversion.cpp

mlir/test/Conversion/SecIntegerToArith/
    unary.mlir
    add-sub.mlir
    multiply.mlir
    divide-rem.mlir
    bitwise.mlir
    shifts.mlir
    comparisons.mlir
    signedness.mlir
    wide.mlir
    storage.mlir
    calls.mlir
    foreign-call.mlir
    provenance.mlir
    invalid-preconditions.mlir
    idempotent.mlir
    no-poison-flags.mlir
```

---

# 14. Conversion target

After successful Package 8 conversion these operations are illegal:

```text
sec.int.unary_plus
sec.int.neg_checked
sec.int.binary_checked
sec.int.bit_not
sec.int.bitwise
sec.int.shift_checked
sec.int.cmp
```

These remain legal:

```text
sec.fail.arithmetic
sec.call.foreign
remaining Sec string/decimal/storage/nominal operations
```

The Sec dialect is not globally illegal.

---

# 15. Integer TypeConverter

Use a TypeConverter that converts plain integer signedness as defined above.

It must convert compatible types in:

```text
func.func inputs/results
block arguments
func.call operands/results
cf branch/return plumbing
rank-zero Sec-origin memrefs
sec.call.direct
sec.call.foreign
plain integer arith.constant
schema-v4 integer ops
```

It must not recursively convert nominal Sec types.

---

# 16. Function metadata preservation

When converting `func.func`, preserve:

```text
symbol
sec.function_id
sec.source_name
sec.extern
sec.unsafe
sec.link_name
sec.abi
parameter names
argument/result attributes
sec.scalar_kind
location
```

Do not rebuild extern declarations without provenance.

---

# 17. Foreign-call rule

`sec.call.foreign` remains a Sec operation.

Its plain builtin integer operand/result types may become signless.

The callee extern function retains `sec.scalar_kind` on arguments/results.

Package 8 must not convert:

```text
sec.call.foreign -> func.call
```

---

# 18. Rank-zero memref conversion

Sec-origin automatic scalar storage may already be:

```text
memref<siN>
memref<uiN>
```

Convert to:

```text
memref<iN>
```

for rank-zero memrefs carrying Sec storage provenance.

Convert:

```text
memref.alloca
memref.load
memref.store
```

consistently.

Preserve storage and scalar provenance.

Do not make Package 8 a general arbitrary MemRef conversion pass.

---

# 19. Integer constants

Convert plain builtin integer `arith.constant` values whose types become
signless.

Preserve exact APInt bits.

Examples:

```text
ui8 255 -> i8 0xff
si8 -1  -> i8 0xff
```

Named/distinct `sec.const.int` remains unchanged.

---

# 20. No overflow assumption flags

Package 8 must not emit:

```text
overflow<nsw>
overflow<nuw>
nneg
exact
```

as correctness assumptions.

Sec checked failure is explicit and deterministic.

---

# 21. Unary plus

Lower:

```text
sec.int.unary_plus
```

to identity.

---

# 22. Bitwise lowering

Map:

```text
bit_not -> arith.xori with all-ones APInt
and     -> arith.andi
or      -> arith.ori
xor     -> arith.xori
```

All operands/results use signless `iN`.

---

# 23. Comparison lowering

Equality:

```text
eq -> arith.cmpi eq
ne -> arith.cmpi ne
```

Signed ordered:

```text
lt -> slt
le -> sle
gt -> sgt
ge -> sge
```

Unsigned ordered:

```text
lt -> ult
le -> ule
gt -> ugt
ge -> uge
```

Determine signedness from the original schema-v4 operand type before type
conversion.

---

# 24. Checked negation

For signed `iN`:

```text
zero = 0
minimum = signed minimum APInt

failed = arith.cmpi eq, value, minimum
result = arith.subi zero, value
```

No overflow flags.

The modulo candidate is ignored on the failure path.

---

# 25. Unsigned checked addition

Preferred:

```text
result, failed = arith.addui_extended left, right
```

The overflow bit is exactly the Sec unsigned-add failure flag.

---

# 26. Unsigned checked subtraction

Preferred:

```text
result, failed = arith.subui_extended left, right
```

The borrow bit is exactly the Sec unsigned-underflow failure flag.

---

# 27. Signed checked addition

For width `N`, sign-extend both operands to `i(N+1)`.

Compute:

```text
wide = arith.addi leftWide, rightWide
result = arith.trunci wide to iN
```

Failure:

```text
wide < signedMinimum(N)
or
wide > signedMaximum(N)
```

The widened sum is exact.

No overflow flags.

---

# 28. Signed checked subtraction

Use the same exact `i(N+1)` signed widening.

Compute subtraction, truncate for candidate result, and compare the widened
difference against the signed N-bit range.

---

# 29. Unsigned checked multiplication

Preferred:

```text
low, high = arith.mului_extended left, right
failed = high != 0
result = low
```

---

# 30. Signed checked multiplication

Preferred:

```text
low, high = arith.mulsi_extended left, right
signFill = arith.shrsi low, (N - 1)
failed = high != signFill
result = low
```

The fixed count `N-1` is valid.

Equivalent exact `i(2N)` widening is acceptable.

---

# 31. Division safety

Never execute raw Arith division on an invalid Sec input.

Compute `failed` first, select a safe divisor, then execute the lower division.

---

# 32. Unsigned division

```text
failed = right == 0
safeRight = select failed, 1, right
result = arith.divui left, safeRight
```

Raw divisor is never zero.

---

# 33. Signed division

```text
divByZero = right == 0
signedOverflow = left == signedMinimum(N) && right == -1
failed = divByZero || signedOverflow

safeRight = select failed, 1, right
result = arith.divsi left, safeRight
```

No `exact`.

---

# 34. Unsigned remainder

```text
failed = right == 0
safeRight = select failed, 1, right
result = arith.remui left, safeRight
```

---

# 35. Signed remainder

Use the same Sec failure set as signed division:

```text
right == 0
or
left == signedMinimum(N) && right == -1
```

Then:

```text
safeRight = select failed, 1, right
result = arith.remsi left, safeRight
```

---

# 36. Shift-count normalization

Let:

```text
N = shifted value width
M = count width
K = max(N + 1, M + 1)
```

Extend count to `iK` using its original semantic signedness.

Failure:

```text
signed count < 0
or
count >= N
```

Create:

```text
safeCountWide = select invalid, 0, countWide
safeCount = truncate safeCountWide to iN
```

Every raw Arith shift uses `safeCount`.

---

# 37. Unsigned left shift

```text
result = arith.shli value, safeCount
failed = invalid
```

No overflow flags.

High-bit discard is Sec-defined behavior.

---

# 38. Unsigned right shift

```text
result = arith.shrui value, safeCount
failed = invalid
```

---

# 39. Signed right shift

```text
result = arith.shrsi value, safeCount
failed = invalid
```

No `exact`.

---

# 40. Signed left shift

Use exact `i(2N)` widening.

```text
valueWide = arith.extsi value to i(2N)
countWide = arith.extui safeCount to i(2N)
wideShift = arith.shli valueWide, countWide
```

Because `safeCount < N`, the raw widened shift is defined.

Failure:

```text
invalid
or
wideShift < signedMinimum(N)
or
wideShift > signedMaximum(N)
```

Candidate:

```text
result = arith.trunci wideShift to iN
```

No shift overflow flags.

---

# 41. Existing failure CFG is preserved

Package 8 replaces only the semantic operation's result/failure values.

Existing:

```text
cf.cond_br %failed, ^failure, ^success
```

remains.

`sec.fail.arithmetic` remains unchanged.

No new failure block is created.

---

# 42. Pre-lowering guard verification

Package 8 must call/share the Package 7 checked-guard verifier before rewriting
checked operations.

Malformed checked CFG must fail before semantic integer ops disappear.

Do not depend only on external manual pass ordering.

---

# 43. Signedness provenance validation

Every extern plain integer argument/result that becomes signless must retain:

```text
sec.scalar_kind
```

If it is missing, fail rather than erase the only remaining ABI-relevant source
classification.

---

# 44. Package 6 integration correction

Update Package 6/high-level emission so `sec.scalar_kind` exists before semantic
scalar representation is collapsed.

At minimum cover:

```text
int32
uint32
int128
uint256
byte
char
rune
```

---

# 45. No general numeric cast semantics

The `extsi`, `extui`, and `trunci` operations generated internally by Package 8
do not implement Sec source numeric casts.

They are lowering implementation details only.

---

# 46. Pass registration

Register:

```bash
--sec-lower-checked-integers
```

and named pipeline:

```bash
--sec-lower-integer-core
```

Conceptual pipeline:

```text
sec-verify-checked-integer-guards
sec-lower-scalar-core
sec-lower-checked-integers
```

---

# 47. Idempotence

After one successful Package 8 pass:

```text
no sec.int.* ops remain
plain converted integer core types are signless
```

A second pass must not change semantics or fail merely because lowering is
already complete.

---

# 48. Required arithmetic boundary tests

For every width:

```text
8
16
32
64
128
256
```

test signed:

```text
minimum
minimum + 1
-1
0
1
maximum - 1
maximum
```

and unsigned:

```text
0
1
maximum - 1
maximum
```

Use generated matrices where practical.

---

# 49. Checked add/sub tests

Cover signed min/max boundaries and unsigned overflow/underflow.

Explicitly include:

```text
int128
int256
uint128
uint256
```

---

# 50. Multiplication tests

Signed:

```text
max * 1
min * 1
min * -1
positive overflow
negative overflow
zero
```

Unsigned:

```text
max * 1
max * 2
zero
```

Across all active widths.

---

# 51. Division/remainder tests

Signed division:

```text
7 / 3 == 2
-7 / 3 == -2
7 / -3 == -2
-7 / -3 == 2
zero divisor fails
minimum / -1 fails
```

Signed remainder:

```text
7 % 3 == 1
-7 % 3 == -1
7 % -3 == 1
-7 % -3 == -1
zero divisor fails
minimum % -1 follows Sec checked failure
```

Unsigned normal and zero-divisor cases.

Inspect IR to prove raw divisor is the selected safe divisor.

---

# 52. Shift tests

For every active width:

```text
count 0
count N-1
count N
negative signed count
different count width
count wider than shifted value
count narrower than shifted value
```

Verify raw shift always receives validated safe count.

---

# 53. Compare tests

Use bit patterns whose signed and unsigned interpretation differs.

Example `i8 0xff`:

```text
signed = -1
unsigned = 255
```

Verify signed predicates use `s*` and unsigned predicates use `u*`.

---

# 54. Provenance tests

Verify:

```text
extern int32 -> i32 + scalar_kind int32
extern uint32 -> i32 + scalar_kind uint32
int128/uint128 -> i128 with distinct provenance
char and rune provenance survives
byte provenance survives
missing extern provenance rejects lowering
```

---

# 55. Storage tests

Sec-origin rank-zero:

```text
memref<si32> -> memref<i32>
memref<ui32> -> memref<i32>
memref<si128> -> memref<i128>
memref<ui256> -> memref<i256>
```

Bool storage remains:

```text
memref<i8>
```

Preserve provenance.

---

# 56. Poison/UB audit tests

Package 8 output must not use:

```text
overflow<nsw>
overflow<nuw>
nneg
exact
```

for checked integer correctness.

Prove:

```text
raw div/rem divisor comes from safe select
raw shift count comes from safe-count normalization
```

---

# 57. Differential APInt tests

Use `llvm::APInt` reference logic across representative/random values for:

```text
checked add
checked subtract
checked multiply
checked negate
signed/unsigned divide
signed/unsigned remainder
signed/unsigned shifts
comparisons
```

Widths:

```text
8
16
32
64
128
256
```

Do not use host integer arithmetic as the wide reference.

---

# 58. Integration tests

Pipeline:

```text
Sec source
    ↓
Sema
    ↓
Semantic IR
    ↓
Sec MLIR schema v4
    ↓
checked guard verifier
    ↓
scalar core
    ↓
checked integer Arith lowering
    ↓
verified mixed standard/Sec MLIR
```

Required examples:

```text
int32 add
uint32 subtract
int64 divide
uint64 remainder
int128 multiply
int256 signed left shift
uint128 logical right shift
uint256 compare
target-sized int on 32-bit target
target-sized int on 64-bit target
nested arithmetic with calls
signed and unsigned comparisons in if
```

---

# 59. No `sec.int.*` after pass

Successful Package 8 output for the supported domain contains none of:

```text
sec.int.unary_plus
sec.int.neg_checked
sec.int.binary_checked
sec.int.bit_not
sec.int.bitwise
sec.int.shift_checked
sec.int.cmp
```

`sec.fail.arithmetic` remains.

---

# 60. Optimization boundary

Correctness must not depend on:

```text
canonicalize
CSE
SCCP
inlining
DCE
```

Emit correct total computations first.

Optimization may simplify them later.

---

# 61. Real MLIR test commands

Representative:

```bash
cmake --build build/sec-mlir
cmake --build build/sec-mlir --target check-sec-mlir

sec-mlir-opt input.mlir \
    --sec-lower-integer-core \
    --verify-each \
    -o output.mlir
```

Do not use unregistered-dialect mode.

---

# 62. Go regression

Run:

```bash
go test ./...
```

Existing dump/backend commands retain their previously defined stage meanings.

Do not silently make `emit-sec-mlir` emit post-P8 Arith MLIR.

---

# 63. Architecture rules

Non-negotiable:

```text
All active 128/256-bit integer types are fully supported.

Checked semantics are verified before lowering.

Sec failure is never represented by poison.

Raw div/rem never receives invalid divisor.

Raw shift never receives invalid count.

Unsigned left shift may truncate high bits without overflow failure.

Signed left shift checks mathematical representability.

Signed right shift is arithmetic.

Unsigned right shift is logical.

Ordered comparisons select signed/unsigned predicates explicitly.

Signless normalization occurs only after semantic operator selection is explicit.

ABI-relevant scalar provenance survives signless normalization.

Named/distinct identity is not erased.

Foreign calls remain Sec operations.

sec.fail.arithmetic remains explicit.

No LLVM dialect is generated.
```

---

# 64. Acceptance criteria

Package 8 is complete only when:

```text
[ ] previous regressions remain green
[ ] rules/sec_mlir_lowering.md updated to v4
[ ] Sec dialect schema remains v4
[ ] Package 6 emits scalar provenance before representation erasure
[ ] extern plain integer params/results retain sec.scalar_kind
[ ] storage provenance retains sec.scalar_kind
[ ] --sec-lower-checked-integers registered
[ ] --sec-lower-integer-core registered
[ ] checked guards verify before lowering
[ ] si/ui widths 8/16/32/64/128/256 normalize to signless
[ ] nominal types are not recursively normalized
[ ] function/block/call types are consistent
[ ] foreign calls remain
[ ] Sec-origin integer memrefs normalize
[ ] integer constants preserve exact APInt bits
[ ] all schema-v4 sec.int ops lower
[ ] no poison assumption flags are emitted
[ ] division/remainder are total through safe substitution
[ ] shifts are total through safe-count normalization
[ ] comparisons preserve signedness semantics
[ ] failure CFG remains
[ ] sec.fail.arithmetic remains
[ ] no sec.int.* op remains
[ ] pass is idempotent
[ ] all active widths pass
[ ] provenance tests pass
[ ] differential APInt tests pass
[ ] check-sec-mlir passes
[ ] go test ./... passes
[ ] no LLVM dialect operation is generated
[ ] legacy paths remain operational
```

---

# 65. Required implementation report

Codex must report:

```text
1. repository HEAD implemented against
2. previous package status
3. files added
4. files modified
5. Package 6 provenance correction
6. scalar provenance representation
7. pass/pipeline registration
8. TypeConverter rules
9. function/block/call conversion
10. memref conversion
11. signed add/sub algorithm
12. unsigned add/sub algorithm
13. multiply algorithm
14. div/rem safe-substitution algorithm
15. shift-count normalization
16. signed-left shift algorithm
17. comparison mapping
18. poison/UB audit
19. wide integer tests
20. provenance tests
21. differential APInt tests
22. CMake commands
23. exact LLVM/MLIR version
24. check-sec-mlir result
25. go test ./... result
26. end-to-end results
27. deviations
28. Package 9 recommendation
```

---

# 66. Package 9 boundary

Preferred next package when the arithmetic-error model is ready:

```text
Arithmetic Failure Endpoint and Fallible Integer Flow
```

Likely scope:

```text
canonical ArithmeticError representation
ordinary sec.fail.arithmetic runtime/panic lowering
try-selected checked integer failure
Result/ArithmeticError propagation
no-panic interaction
runtime helper ABI contract
failure provenance
```

If the arithmetic-error rules are not yet frozen, use:

```text
Float Semantic Operations
```

as Package 9 instead.

Do not combine both areas unless the failure/error model is already normative.

---

# 67. Upstream MLIR references

Implement against the exact project-selected MLIR version.

Relevant upstream references:

```text
https://mlir.llvm.org/docs/Dialects/ArithOps/
https://mlir.llvm.org/docs/DialectConversion/
```
