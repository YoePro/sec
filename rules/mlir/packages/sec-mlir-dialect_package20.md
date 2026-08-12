# Sec MLIR Program - Implementation Package 20

Created: 2026-08-11  
Last updated: 2026-08-11  
Revision: 1  
Sec language version: 0.1  
## Package status

Implementation package for the Sec compiler.

Package ID: `SEC-MLIR-P20`  
Package title: `Floating-Point Semantic Operations and Arith Lowering`  
Repository: `https://github.com/YoePro/sec`  
Repository branch: `main`  
Repository sync commit used for this package: `e0af215`  
Local predecessors: corrected `SEC-MLIR-P18`, `SEC-MLIR-P19`, and the earlier local package chain  
Repository sync date: `2026-08-11`  
Semantic IR version before package: `1`  
Semantic IR version after package: `1`  
Sec MLIR dialect schema before package: `15`  
Sec MLIR dialect schema after package: `16`  
Sec MLIR lowering specification before package: `15`  
Sec MLIR lowering specification after package: `16`

Package 20 completes the target-independent binary floating-point core for:

```text
float
float32
float64
```

It defines and implements:

```text
directly rounded floating literals
binary32/binary64 semantic formats
ordinary floating arithmetic
truncation-based remainder
unary sign operations
IEEE-style equality and ordering
NaN/infinity/subnormal classification
absolute value
minimum/maximum
checked clamp
floor/ceiling/truncate/round
associated floating constants
checked exact numeric conversion
float-to-bool conversion
constant folding parity
standard Arith/Math lowering
no-fast-math verification
```

It also closes the long-standing Package 4/5/6 deferral:

```text
sec.const.float -> arith.constant
```

after the floating literal rounding rule is fixed.

Package 20 does not implement decimal arithmetic, `decimal128` arithmetic,
floating formatting/string allocation, or `SquareRoot`.

---

# 1. Normative authority

Implementation follows:

```text
rules/types.md
rules/operators.md
rules/core-library.md
rules/runtime_checks.md
rules/layout.md
rules/compiler_known_members.md
    ↓
local P2-P19 Semantic IR / Sec MLIR amendments
    ↓
rules/semantic_ir.txt
rules/sec_mlir.md
rules/sec_mlir_dialect.md
rules/sec_mlir_lowering.md
    ↓
implementation package
    ↓
implementation
```

Before implementation:

1. apply `sec_float_numeric_sync_package20.md`;
2. apply `sec_semantic_ir_float_package20.md` to `rules/semantic_ir.txt`;
3. update `rules/sec_mlir_dialect.md` with `sec_mlir_dialect_package20.md`;
4. update `rules/sec_mlir_lowering.md` with `sec_mlir_lowering_package20.md`.

No new floating literal syntax is introduced.

---

# 2. Repository baseline

P20 is based on:

```text
e0af215
feat(frontend): unify compiler-known collections and synchronize rulebooks
```

The package also assumes the user's corrected local P18 and installed P19
semantics.

Do not reapply an older P18 file over the corrected local version.

---

# 3. Active wide-type invariant

These are active builtins:

```text
int128
int256
uint128
uint256
decimal128
```

P20 uses all active integer widths in integer/floating conversion coverage.

P20 does not implement decimal/decimal128 conversion semantics; those are P21.

---

# 4. Binary floating source types

Canonical:

```text
float
float32
float64
```

Semantic formats:

```text
float32 -> IEEE-compatible binary32
float64 -> IEEE-compatible binary64
float   -> same semantic numeric width and native layout as float64
```

`float` and `float64` remain distinct source type identities even though their
current physical format is the same.

P8 `sec.scalar_kind` provenance continues to distinguish them.

---

# 5. Binary format facts

P20 uses:

```text
binary32:
    total bits       32
    significand bits 24
    exponent bits    8
    exponent bias    127

binary64:
    total bits       64
    significand bits 53
    exponent bits    11
    exponent bias    1023
```

Gradual underflow/subnormal values are part of Sec semantics.

---

# 6. Default floating rounding mode

Ordinary Sec binary floating arithmetic uses:

```text
round to nearest, ties to even
```

for every source arithmetic operation independently.

No dynamic floating-point environment or source-visible rounding-mode state is
part of Sec 0.1.

A target that cannot preserve this mode must use a semantics-preserving helper
or reject the unsupported operation/profile.

---

# 7. No contraction

Source:

```sec
a * b + c
```

means:

```text
rounded multiplication
then
rounded addition
```

unless a future explicit fused operation is selected by source semantics.

The compiler must not silently contract this expression into FMA.

---

# 8. No unsafe fast-math

Compiler-generated ordinary floating operations use:

```text
fastmath = none
```

Forbidden implicit assumptions include:

```text
reassoc
nnan
ninf
nsz
arcp
contract
afn
fast
```

No build profile may silently enable them.

---

# 9. No denormal flushing

Ordinary Sec semantics preserve subnormal values.

Do not insert `arith.flush_denormals` for ordinary Sec floating semantics.

If a target cannot provide the required behavior, use a preserving helper or
reject the target/profile operation.

---

# 10. Floating exception flags

Sec 0.1 does not expose ambient IEEE floating exception flags/traps as ordinary
language state.

Floating division by zero is a value operation, not `DivisionByZeroError`.

---

# 11. Literal family and context shaping

The binary floating family suffix is:

```text
g
```

Examples:

```sec
let value := 1.5g
let whole := 10g
let small: float32 := 1.5g
let wide: float64 := 1.5g
```

A `g` suffix selects the binary floating family, not a width.

An unsuffixed literal may also be shaped directly by an explicit float context:

```sec
let value: float32 := 0.1
```

That is literal shaping, not a decimal runtime conversion.

---

# 12. Exact-source literal preservation

Before final float lowering, preserve:

```text
exact source lexeme
resolved source numeric form
resolved floating target type
source location
```

P4 `sec.const.float` already retains the source lexeme.

P20 consumes it.

---

# 13. Direct literal rounding

A floating literal is converted directly from its exact source numeric value to:

```text
binary32
or
binary64
```

using round-to-nearest, ties-to-even.

There must be exactly one semantic rounding step.

---

# 14. No double rounding

Forbidden for `float32`:

```text
source exact value
    -> binary64
    -> binary32
```

Required:

```text
source exact value
    -> binary32
```

The same rule applies to integer-form `g` literals.

---

# 15. Integer-form `g` literals

Examples:

```sec
0x10g
0b101g
10g
```

A base-prefixed spelling denotes the exact mathematical integer value, then
rounds once into the resolved binary floating format.

It is not C hexadecimal floating syntax.

---

# 16. Literal overflow and underflow

A finite source literal outside the finite range of the resolved floating type
is a compile-time error.

Do not silently shape it to infinity.

Very small finite literals use normal nearest-even conversion and may become:

```text
normal
subnormal
signed zero
```

No flush-to-zero rule is added.

---

# 17. Negative zero literal

Unary minus remains a semantic operation.

Conceptually:

```sec
-0.0g
```

is positive-zero literal shaping followed by floating negation and produces
negative zero.

---

# 18. Canonical float constant payload

Semantic IR must retain a format-precise value.

Recommended representation:

```go
type BinaryFloatFormat string

const (
    BinaryFloat32 BinaryFloatFormat = "binary32"
    BinaryFloat64 BinaryFloatFormat = "binary64"
)

type BinaryFloatConstant struct {
    Format       BinaryFloatFormat
    Bits         uint64
    SourceLexeme string
}
```

For binary32, only the low 32 bits are significant.

An equivalent immutable APFloat-style representation is acceptable.

A host `float64` is not the canonical semantic representation of a binary32
constant.

---

# 19. Frontend parsing rule

The Go frontend may use a standard parser only if it parses directly at the
resolved bit width and returns the canonical bit pattern.

Required:

```text
float32 target -> direct 32-bit parse/round
float64 target -> direct 64-bit parse/round
```

Forbidden:

```text
parse to host float64
then narrow to float32
```

---

# 20. Unary plus and negation

Unary plus:

```text
preserves type/value
preserves signed zero
preserves NaN category
```

It may canonicalize to SSA identity.

Negation toggles the sign according to the selected binary format:

```text
+0 <-> -0
+infinity <-> -infinity
NaN remains NaN
```

No arithmetic failure.

---

# 21. Arithmetic

For the Sema-resolved common floating type:

```text
+  add
-  subtract
*  multiply
/  divide
```

all use:

```text
nearest-even rounding
IEEE-compatible NaN/infinity/signed-zero behavior
no integer-style checked overflow
```

Overflow may produce infinity.

---

# 22. Floating division

Examples:

```text
finite nonzero / ±0 -> signed infinity
0 / 0               -> NaN
infinity / infinity -> NaN
```

No runtime Result or arithmetic panic is generated solely for a zero divisor.

---

# 23. Floating remainder

`%` uses truncation-based floating remainder.

For finite operands:

```text
q = trunc(left / right)
r = left - q * right
```

Special cases follow `operators.md`.

This is not nearest-integer IEEE remainder.

---

# 24. Floating equality

Canonical:

```text
NaN == NaN -> false
NaN != NaN -> true
+0 == -0   -> true
+0 != -0   -> false
```

---

# 25. Floating ordering

For:

```text
<
<=
>
>=
```

if either operand is NaN:

```text
false
```

Positive and negative zero compare equal.

No total order is implied.

---

# 26. Canonical compare mapping

After scalar resolution:

```text
Sec == -> arith.cmpf oeq
Sec != -> arith.cmpf une
Sec <  -> arith.cmpf olt
Sec <= -> arith.cmpf ole
Sec >  -> arith.cmpf ogt
Sec >= -> arith.cmpf oge
```

`one` is forbidden for source `!=`.

---

# 27. Legacy comparator correction

While the legacy MLIR path remains operational, P20 must correct floating `!=`:

```text
ONE -> UNE
```

This is a semantic correctness fix.

---

# 28. Core floating properties

P20 implements the existing core properties:

```text
min
max
epsilon
infinity
negativeInfinity
nan
```

Definitions:

```text
min:
    most negative finite value

max:
    largest positive finite value

epsilon:
    difference between 1 and the next representable value greater than 1

infinity:
    positive infinity

negativeInfinity:
    negative infinity

nan:
    deterministic canonical quiet NaN
```

For epsilon:

```text
binary32 -> 2^-23
binary64 -> 2^-52
```

It is not the smallest positive subnormal.

---

# 29. `Abs`

`Abs()` is total:

```text
finite negative -> magnitude
-0              -> +0
+0              -> +0
±infinity       -> +infinity
NaN             -> NaN
```

---

# 30. `Min` and `Max`

P20 defines the core methods as NaN-propagating.

`Min`:

```text
if either operand is NaN -> NaN
otherwise numeric minimum
Min(-0, +0) -> -0
```

`Max`:

```text
if either operand is NaN -> NaN
otherwise numeric maximum
Max(-0, +0) -> +0
```

---

# 31. `Clamp`

Existing signature:

```sec
fn Clamp(minimum: T, maximum: T) Result[T, RangeError]
```

Invalid bounds:

```text
minimum is NaN
maximum is NaN
minimum > maximum
```

Failure:

```text
RangeError.InvalidRange
```

With valid bounds, a NaN input value produces `Ok(NaN)`.

---

# 32. Classification methods

P20 implements:

```text
IsFinite
IsInfinite
IsNaN
IsNormal
IsSubnormal
```

as total allocation-free operations.

Binary classification:

```text
finite:
    exponent != all ones

infinite:
    exponent == all ones
    fraction == zero

nan:
    exponent == all ones
    fraction != zero

normal:
    exponent != zero
    exponent != all ones

subnormal:
    exponent == zero
    fraction != zero
```

---

# 33. Classification lowering strategy

Use:

```text
arith.bitcast
integer masks
integer comparisons
```

or an equivalent target-independent standard-MLIR sequence.

No runtime library is semantically required.

---

# 34. Rounding methods

P20 defines:

```text
Floor:
    toward negative infinity

Ceiling:
    toward positive infinity

Truncate:
    toward zero

Round:
    nearest integral floating value, ties to even
```

All return the same floating type.

NaN and infinities remain NaN/infinity.

Signed zero is preserved.

---

# 35. Standard rounding lowering

After scalar resolution:

```text
Abs      -> math.absf
Floor    -> math.floor
Ceiling  -> math.ceil
Truncate -> math.trunc
Round    -> math.roundeven
```

Compiler-generated Math operations use no fast-math assumptions.

---

# 36. Standard Min/Max lowering

P20's semantics map to:

```text
Min -> arith.minimumf
Max -> arith.maximumf
```

Do not use:

```text
arith.minnumf
arith.maxnumf
```

because those suppress one NaN operand.

---

# 37. `SquareRoot` boundary

`SquareRoot()` remains part of the core surface but P20 does not lower it.

Its detailed `Result[T, MathError]` behavior belongs to a later core-math
package.

Do not infer source semantics merely from `math.sqrt`.

---

# 38. Formatting boundary

P20 does not implement:

```text
ToString
formatted ToString
float parsing
locale formatting
```

Those depend on string/materialization/formatting semantics.

---

# 39. Numeric conversion core error

Add:

```sec
enum NumericConversionError {
    OutOfRange
    PrecisionLoss
    NotFinite
}
```

This is a standard core error.

---

# 40. Why a dedicated conversion error exists

A checked builtin numeric conversion may fail because of:

```text
range
precision
non-finite source
```

Sec does not infer hidden error unions.

One exact error type is therefore required for stable `try` semantics.

---

# 41. Error priority

When more than one category might conceptually apply:

```text
1. NotFinite
2. OutOfRange
3. PrecisionLoss
```

Optimization must preserve the selected error identity.

---

# 42. P20 conversion scope

P20 covers:

```text
builtin signed integers -> float family
builtin unsigned integers -> float family
float family -> builtin signed integers
float family -> builtin unsigned integers
float family -> float family
float family -> bool
```

Integer coverage includes:

```text
int
uint
8/16/32/64/128/256 fixed widths
```

after target-sized resolution where needed.

Decimal conversions are P21.

---

# 43. Explicit conversion syntax

Existing syntax remains:

```sec
TargetType(value)
```

Potentially failing runtime conversion requires:

```sec
try TargetType(value)
```

For P20 numeric conversions, the exact error type is:

```text
NumericConversionError
```

---

# 44. Proven conversion

If Sema proves exactness and range:

```text
no runtime failure branch
```

Semantic IR still records the resolved conversion plan.

---

# 45. Float32 widening

These are exact and infallible:

```text
float32 -> float64
float32 -> float
```

They preserve every binary32 numeric value.

---

# 46. `float` and `float64`

Current Sec 0.1 gives both the binary64 numeric format.

Explicit conversion between them is exact and infallible.

Source type identity still changes.

---

# 47. Float64 to float32

Finite input:

```text
outside finite binary32 range
    -> OutOfRange

inside range but not exactly representable
    -> PrecisionLoss

exactly representable
    -> success
```

Infinity maps to infinity.

NaN maps to NaN.

NaN payload preservation is not a P20 source guarantee.

---

# 48. Narrow-float runtime check

Canonical:

```text
if source is NaN:
    success target NaN

if source is infinity:
    success same-signed target infinity

candidate = nearest-even narrowing
if finite source became infinity:
    OutOfRange

widened = exact widening of candidate
if widened != source:
    PrecisionLoss

otherwise:
    success candidate
```

Signed zero succeeds.

---

# 49. Integer to float

Typed integer-to-float conversion succeeds only when the mathematical integer is
exactly representable.

Failure:

```text
outside finite target range -> OutOfRange
rounding would be required  -> PrecisionLoss
```

No implicit rounding is allowed for an already typed runtime integer.

---

# 50. Integer-to-float exactness

For nonzero magnitude:

```text
bitLength = floor(log2(magnitude)) + 1
precision = 24 or 53

if bitLength <= precision:
    exact
else:
    low (bitLength - precision) magnitude bits must all be zero
```

Range must also fit the largest finite target value.

This algorithm applies to 128/256-bit integers.

---

# 51. Float to integer

Requires:

```text
finite
in target range
integral
```

Failure:

```text
NaN or infinity -> NotFinite
out of range    -> OutOfRange
fractional      -> PrecisionLoss
```

---

# 52. Safe signed range guard

For signed N-bit target:

```text
-2^(N-1) <= value < 2^(N-1)
```

for the already finite source.

Use exact power-of-two bounds to avoid rounded maximum ambiguity.

---

# 53. Safe unsigned range guard

For unsigned N-bit target:

```text
0 <= value < 2^N
```

A bound beyond every finite source value may be proven redundant.

---

# 54. Integral guard

After finite/range checks:

```text
math.trunc(value) == value
```

using ordered equality.

Only then may the final integer conversion execute.

---

# 55. Float-to-bool

Existing numeric-to-bool semantics apply:

```text
+0 -> false
-0 -> false
every other value -> true
```

Therefore:

```text
NaN -> true
±infinity -> true
```

This conversion is explicit and infallible.

Canonical lowering:

```text
arith.cmpf une value, +0.0
```

---

# 56. Numeric conversion reason domain

Semantic IR adds:

```text
None
OutOfRange
PrecisionLoss
NotFinite
```

Invariant:

```text
failed == false <=> reason == None
failed == true  <=> reason != None
```

---

# 57. Naked `try`

Naked propagation of a checked P20 conversion requires:

```text
Result[U, NumericConversionError]
```

with the exact error type.

No hidden wrapper or inferred error union.

---

# 58. Local handlers

Existing P10/P11 local handler semantics apply.

Example:

```sec
let value := try float32(runtimeValue) {
    Err(NumericConversionError.OutOfRange) => fallback
    Err(NumericConversionError.PrecisionLoss) => fallback
    Err(NumericConversionError.NotFinite) => fallback
}
```

No conversion-specific handler engine.

---

# 59. Compile-time failure

If a constant checked conversion is known to fail:

```text
compile-time diagnostic
```

Do not emit a guaranteed runtime Err path.

---

# 60. Sema conversion plan

Recommended:

```go
type NumericConversionKind string

const (
    NumericConvertFloatWiden      NumericConversionKind = "float-widen"
    NumericConvertFloatNarrow     NumericConversionKind = "float-narrow"
    NumericConvertSignedToFloat   NumericConversionKind = "signed-to-float"
    NumericConvertUnsignedToFloat NumericConversionKind = "unsigned-to-float"
    NumericConvertFloatToSigned   NumericConversionKind = "float-to-signed"
    NumericConvertFloatToUnsigned NumericConversionKind = "float-to-unsigned"
    NumericConvertFloatToBool     NumericConversionKind = "float-to-bool"
)

type ResolvedNumericConversionPlan struct {
    Kind         NumericConversionKind
    SourceType   Type
    TargetType   Type
    ProvenExact  bool
    RuntimeCheck bool
    ErrorType    Type
    SourceBits   int
    TargetBits   int
}
```

Equivalent existing compiler data may be reused.

The builder must not rediscover conversion semantics.

---

# 61. Read-only query

Recommended:

```go
func (a *Analyzer) ResolvedNumericConversionPlanOf(
    expr ast.Expression,
) (ResolvedNumericConversionPlan, bool)
```

The query does not mutate Sema.

---

# 62. Core intrinsic IDs

Compiler-known/core member resolution should expose stable IDs for:

```text
FloatMinValue
FloatMaxValue
FloatEpsilon
FloatInfinity
FloatNegativeInfinity
FloatNaN

FloatAbs
FloatMin
FloatMax
FloatClamp

FloatIsFinite
FloatIsInfinite
FloatIsNaN
FloatIsNormal
FloatIsSubnormal

FloatFloor
FloatCeiling
FloatTruncate
FloatRound
```

Use the existing compiler-known registry architecture.

---

# 63. Semantic IR operations

P20 adds/refines:

```text
FloatUnaryPlusOp
FloatNegOp
FloatBinaryOp
FloatCompareOp
FloatAbsOp
FloatMinOp
FloatMaxOp
FloatClampCheckedOp
FloatClassifyOp
FloatRoundOp
FloatPropertyOp

NumericConvertExactOp
NumericConvertCheckedOp
NumericConversionErrorFromReasonOp
FloatToBoolOp
```

P2 `ConstFloatOp` is refined by P20 constant rules.

---

# 64. `FloatBinaryOp`

Kinds:

```text
add
sub
mul
div
rem
```

Required semantic policy:

```text
rounding = nearest-even
fast_math = none
```

For remainder:

```text
remainder_kind = truncation
```

---

# 65. `FloatCompareOp`

Predicates:

```text
eq
ne
lt
le
gt
ge
```

Result:

```text
bool
```

No total-order predicate.

---

# 66. `FloatClassifyOp`

Kinds:

```text
finite
infinite
nan
normal
subnormal
```

Result:

```text
bool
```

---

# 67. `FloatRoundOp`

Kinds:

```text
floor
ceiling
truncate
nearest-even
```

---

# 68. `FloatPropertyOp`

Kinds:

```text
min
max
epsilon
infinity
negative-infinity
nan
```

---

# 69. `FloatClampCheckedOp`

Results:

```text
result
failed
```

Failure always maps to:

```text
RangeError.InvalidRange
```

---

# 70. `NumericConvertExactOp`

Used for proven/intrinsic exact conversion.

One input, one target-typed result.

No error edge.

---

# 71. `NumericConvertCheckedOp`

Results:

```text
candidate
failed
reason
```

The candidate is consumed only on success.

---

# 72. Error mapping op

`NumericConversionErrorFromReasonOp` maps:

```text
OutOfRange    -> NumericConversionError.OutOfRange
PrecisionLoss -> NumericConversionError.PrecisionLoss
NotFinite     -> NumericConversionError.NotFinite
```

`None` is invalid.

---

# 73. Schema version 16

Add:

```text
!sec.numeric_conversion_reason

sec.float.unary_plus
sec.float.neg
sec.float.binary
sec.float.cmp
sec.float.abs
sec.float.minimum
sec.float.maximum
sec.float.clamp_checked
sec.float.classify
sec.float.round
sec.float.property
sec.float.to_bool

sec.numeric.convert_exact
sec.numeric.convert_checked
sec.numeric_conversion_error.from_reason
```

Refine existing:

```text
sec.const.float
```

---

# 74. `sec.const.float` resolution

Before lowering, it carries/has access to:

```text
source lexeme
source scalar kind
resolved binary format
canonical bit pattern
```

Verifier rejects a mismatched pattern.

---

# 75. Standard arithmetic lowering

After P6 scalar resolution:

```text
add -> arith.addf to_nearest_even
sub -> arith.subf to_nearest_even
mul -> arith.mulf to_nearest_even
div -> arith.divf to_nearest_even
rem -> arith.remf
```

No fast-math.

---

# 76. Constant lowering

After verification:

```text
sec.const.float -> arith.constant
```

using the exact canonical bit pattern.

Do not reparse the lexeme in the lowering pass.

---

# 77. Comparison lowering

Exactly:

```text
eq -> oeq
ne -> une
lt -> olt
le -> ole
gt -> ogt
ge -> oge
```

---

# 78. Classification lowering

Use logical equal-width bitcast:

```text
f32 -> i32
f64 -> i64
```

Masks:

```text
binary32:
    exponentMask = 0x7f800000
    fractionMask = 0x007fffff

binary64:
    exponentMask = 0x7ff0000000000000
    fractionMask = 0x000fffffffffffff
```

Then use standard integer Arith.

---

# 79. Checked conversion lowering

Register:

```bash
--sec-lower-checked-numeric-conversions
```

It converts checked P20 conversion semantics into total guard/control-flow
sequences and safe standard conversion operations.

A potentially invalid `fptosi`/`fptoui` must never execute before its guards.

---

# 80. Float core lowering pass

Register:

```bash
--sec-lower-float-core
```

It lowers `sec.const.float` and `sec.float.*` to standard Arith/Math/CF.

Decimal operations remain untouched.

---

# 81. Verifiers

Register:

```bash
--sec-verify-float-semantics
--sec-verify-numeric-conversions
```

The float verifier checks:

```text
format
constant bits
rounding policy
no fast-math
comparison predicates
remainder kind
classification
round method
properties
Clamp contract
```

The conversion verifier checks:

```text
supported source/target
exact versus checked classification
error reason invariant
candidate success dominance
safe cast guards
try error compatibility
```

---

# 82. Constant folding

Compile-time folding must use the same binary formats, rounding and comparison
semantics as runtime lowering.

Do not substitute host behavior where it differs.

---

# 83. Target capabilities

CompilationPlan must expose enough capability to validate:

```text
binary32
binary64
nearest-even
subnormal preservation or preserving helper
comparison semantics
remainder semantics or preserving helper
```

Unsupported targets fail with a target capability diagnostic rather than
silently weakening semantics.

---

# 84. Legacy path parity

While legacy lowering remains supported, patch:

```text
float != uses UNE
float literals round directly to resolved width
float remainder matches P20
no unsafe fast-math
```

The legacy backend is not the semantic authority.

---

# 85. Required literal tests

```text
g suffix family
context float32/float64/float
unsuffixed literal shaped to float
integer-form g literal
base-prefixed g literal
direct f32 rounding
direct f64 rounding
known no-double-rounding cases
subnormal
underflow to signed zero
finite overflow rejected
negative zero
```

---

# 86. Required arithmetic/comparison tests

For f32/f64/float:

```text
add/sub/mul/div/rem
unary plus/negation
normal/subnormal
signed zero
infinity
NaN
overflow/underflow
NaN == NaN false
NaN != NaN true
all ordered NaN comparisons false
+0 == -0
legacy UNE correction
```

---

# 87. Required classification/rounding tests

Classification:

```text
zero
smallest/largest subnormal
smallest normal
largest finite
infinity
multiple NaN bit patterns
```

Rounding:

```text
positive/negative fraction
positive/negative half
ties-to-even
signed zero
infinity
NaN
subnormal
```

---

# 88. Required conversion tests

Float widening/narrowing:

```text
exact
precision loss
finite overflow
subnormal
signed zero
NaN
infinity
```

Integer-to-float:

```text
2^24 boundary
2^24+1
2^53 boundary
2^53+1
int128/uint128
int256/uint256
f32 range overflow
exact powers of two
```

Float-to-integer:

```text
finite integral
fractional
NaN
infinity
signed/unsigned bounds
negative-to-unsigned
subnormal
```

Float-to-bool:

```text
±0 false
finite nonzero true
subnormal true
infinity true
NaN true
```

---

# 89. Required try tests

```text
NumericConversionError identity
exact naked propagation
specific handlers
catch-all
wrong enclosing error rejected
no hidden error union
constant known failure diagnostic
runtime checked failure
proven exact no runtime branch
```

---

# 90. Required standard MLIR output

Tests should cover:

```text
arith.constant
arith.addf
arith.subf
arith.mulf
arith.divf
arith.remf
arith.cmpf
arith.minimumf
arith.maximumf
arith.bitcast
arith.sitofp
arith.uitofp
arith.fptosi
arith.fptoui
arith.extf
arith.truncf

math.absf
math.floor
math.ceil
math.trunc
math.roundeven
```

No LLVM dialect is required by P20.

---

# 91. Explicitly deferred

P20 does not implement:

```text
decimal/decimal128 arithmetic
decimal <-> float conversions
SquareRoot detailed MathError semantics
advanced math
float formatting/string conversion
float parsing
explicit total order
explicit lossy numeric conversions
dynamic rounding environment
source fast-math mode
NaN payload manipulation
LLVM lowering
```

---

# 92. Architecture rules

Non-negotiable:

```text
float32 is binary32.
float64 is binary64.
float has binary64 semantic width/layout in Sec 0.1.

Floating literals round once directly from exact source spelling to resolved
format.

No f64 intermediate is permitted for f32 literal shaping.

Ordinary arithmetic uses nearest-even.
No implicit FMA contraction.
No unsafe fast-math.
No implicit denormal flush.

Floating division by zero is not an error.
Floating remainder is truncation-based.

NaN != NaN is true.
Source float != uses unordered-not-equal.
Ordered NaN comparisons are false.

Min/Max propagate NaN.
Round uses ties-to-even.

Typed numeric conversion is precision-preserving and checked.
Literal shaping is not typed-value conversion.

NumericConversionError is the exact P20 conversion error.

Float-to-int invalid inputs are guarded before final cast.
Integer-to-float succeeds only when exact.
float32 -> float64 is exact.
float64 -> float32 is checked.

P8 scalar provenance is reused.
P17 ownership remains trivial.
P19 allocation context is not required.

No LLVM/backend behavior defines Sec float semantics.
```

---

# 93. Acceptance criteria

Package 20 is complete only when:

```text
[ ] baseline is e0af215 + corrected local P18/P19 chain
[ ] previous package regressions remain green
[ ] float/numeric synchronization applied
[ ] NumericConversionError added to core
[ ] Semantic IR float amendment applied
[ ] schema-v16 rulebook installed
[ ] lowering-v16 rulebook installed
[ ] binary32/binary64 format facts implemented
[ ] nearest-even arithmetic policy implemented
[ ] no-fast-math enforced
[ ] no-denormal-flush enforced
[ ] direct literal rounding implemented
[ ] no f64 intermediate for f32 literal
[ ] sec.const.float lowers to arith.constant
[ ] add/sub/mul/div/rem lower correctly
[ ] compare predicates correct
[ ] legacy ONE -> UNE fix implemented
[ ] floating properties implemented
[ ] Abs/Min/Max/Clamp implemented
[ ] classification implemented
[ ] Floor/Ceiling/Truncate/Round implemented
[ ] ResolvedNumericConversionPlan implemented/read-only
[ ] exact and checked conversion ops implemented
[ ] NumericConversionError mapping implemented
[ ] float narrowing exactness implemented
[ ] int/uint -> float exact checks include 128/256
[ ] float -> int/uint safe checked conversion implemented
[ ] float -> bool implemented
[ ] try/local-handler integration implemented
[ ] float semantic verifier registered
[ ] numeric conversion verifier registered
[ ] checked conversion lowering registered
[ ] float core lowering registered
[ ] constant-fold parity tests pass
[ ] target capability checks implemented
[ ] SquareRoot remains explicitly deferred
[ ] decimal/decimal128 remain active and P20-deferred to P21
[ ] no LLVM dialect is required
[ ] check-sec-mlir passes
[ ] go test ./... passes
[ ] legacy paths remain operational
```

---

# 94. Required implementation report

Codex must report:

```text
1. repository HEAD implemented against
2. corrected local P18/P19 predecessor status
3. previous package status
4. float/numeric normative synchronization
5. files added
6. files modified
7. NumericConversionError implementation
8. float format representation
9. direct literal parser/rounder
10. no-double-rounding tests
11. sec.const.float lowering
12. float arithmetic lowering
13. remainder lowering
14. compare predicates
15. legacy UNE correction
16. no-fast-math enforcement
17. rounding-mode enforcement
18. property constants
19. classification implementation
20. rounding methods
21. Min/Max/Clamp
22. ResolvedNumericConversionPlan
23. exact conversion lowering
24. checked conversion lowering
25. int-to-float exactness algorithm
26. float-to-int guard algorithm
27. float-narrow exactness algorithm
28. float-to-bool
29. Result/try integration
30. schema-v16 types/ops
31. float verifier
32. numeric conversion verifier
33. target capability checks
34. constant folding parity
35. wide-integer conversion tests
36. NaN/signed-zero/subnormal tests
37. unsupported SquareRoot/decimal/string tests
38. CMake commands
39. exact LLVM/MLIR version
40. check-sec-mlir result
41. go test ./... result
42. end-to-end source -> schema-v16 -> Arith/Math results
43. deviations
44. recommendations for Package 21
```

---

# 95. Package 21 boundary

Recommended Package 21:

```text
Decimal and Decimal128 Semantic Operations
```

Scope:

```text
active decimal
active decimal128
exact literal preservation
coefficient/scale semantics
checked add/sub/mul/div/rem
decimal negation/Abs
precision and scale failures
decimal rounding/rescaling
decimal comparison
checked integer/float/decimal conversions
NumericConversionError reuse where appropriate
PrecisionError integration
constant folding parity
no binary-float fallback
no LLVM yet
```

P21 must continue to treat `decimal128` as an active builtin language type.
