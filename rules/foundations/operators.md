# Operators

## Status

This document is the canonical operator rulebook for Sec.

It defines:

- operator inventory;
- precedence;
- associativity;
- evaluation order;
- short-circuit behavior;
- operand compatibility;
- result types;
- arithmetic failure;
- comparison;
- membership;
- assignment;
- contextual operators;
- formatter behavior;
- Semantic IR requirements;
- lowering requirements.

This document must remain synchronized with the actual lexer, parser, AST,
Sema, formatter, Semantic IR, MLIR, and backend implementations.

`default` is a named-type declaration clause, not an operator. Expressions in
an explicit default use the constant-expression subset of the operator rules in
this document. An invalid constant operation invalidates the default, and the
expression may not allocate or depend on runtime state. See
`default_values.md`.

---

# Current implementation status

The Sec compiler already implements a substantial operator foundation.

The current implementation must be preserved where it agrees with this
rulebook and corrected where it does not.

## Implemented

The lexer currently recognizes:

```text
+
-
*
/
%
=
:=
<-
:<-
+=
-=
*=
/=
%=
==
!=
<
<=
>
>=
&&
||
!
&
|
^
~
&=
|=
^=
<<
>>
<<=
>>=
..
..<
...
.
?
=>
```

The lexer uses longest-match behavior for multi-character tokens.

The parser currently implements a Pratt-style expression parser with precedence
levels for:

```text
||
&&
|
^
&
==
!=
<
<=
>
>=
in
<<
>>
+
-
*
/
%
prefix operators
calls and indexes
member access
```

The parser currently supports:

- unary `+`;
- unary `-`;
- logical negation `!`;
- bitwise complement `~`;
- arithmetic infix expressions;
- bitwise infix expressions;
- shift expressions;
- equality and ordered comparison;
- `in`;
- calls;
- indexes;
- slices;
- member access;
- struct literals;
- spread expressions;
- assignment statements;
- compound assignment statements;
- move declaration and move assignment syntax;
- contextual range parsing in selected grammar positions.

The AST currently preserves:

- infix operator spelling;
- prefix operator spelling;
- assignment operator spelling;
- ownership mode for declarations and assignments.

Sema currently implements:

- boolean operands for `&&` and `||`;
- arithmetic numeric checks;
- integer requirements for bitwise operators;
- integer requirements for shifts;
- compile-time rejection of negative and out-of-width shift counts, including
  the representation width of user-defined integer types;
- compile-time rejection of signed left shifts whose known result is outside
  the left operand representation;
- recursive equality comparability for fixed arrays, ordinary structs and
  unions, plus strict nominal compatibility and literal shaping;
- safe-reference equality using compatible reference types, as specialized by
  `reference_model.md`;
- rejection of ordinary equality for slice views, interfaces, functions and
  compiler-known opaque struct types;
- runtime and compile-time string concatenation with the complete direct
  `string`, `char`, and `rune` operand matrix, always producing `string`;
- rejection of hidden conversion for non-text concatenation operands through
  stable diagnostic `S1022`;
- direct `string +=` type validation for `string`, `char`, and `rune` values;
- ordered-comparison validation for compatible numeric operands and matching
  `char`, `rune`, and `string` operands;
- rejection of chained comparisons;
- contextual shaping of character literals;
- range membership;
- fixed-array and slice membership with equality-comparable element validation,
  compatible left-value shaping, and stable diagnostic `S1021`;
- assignment validation;
- compound assignment validation;
- contract checks;
- ownership checks;
- copy and move checks.

The direct LLVM backend already implements lazy short-circuit control flow for:

```text
&&
||
```

The MLIR backend already lowers several:

```text
integer arithmetic operators
floating arithmetic operators
integer remainder
bitwise operators
shifts
comparisons
short-circuit logical operators
range membership
compound assignments
```

## Partially implemented

The following are only partially implemented:

- contextual `x` parsing and fixed matrix/matrix and matrix/vector shape
  validation are implemented, while Semantic IR, lowering and tooling context
  remain;
- overflow checking is incomplete;
- runtime division-by-zero checking is incomplete;
- compile-time shift-count checking is implemented, while deterministic checks
  for dynamically known counts still require runtime-check lowering;
- compile-time signed-left-shift overflow checking is implemented, while
  dynamically known signed overflow still requires runtime-check lowering;
- float comparison predicates do not yet fully match the required NaN rules;
- `%` is accepted broadly by Sema, but float and decimal lowering is incomplete;
- array and slice membership is type-checked, but its exact-once,
  left-to-right short-circuit lowering is not implemented;
- runtime concatenation is type-checked, but allocation-context resolution,
  `try`-selected failure flow, `@noPanic` enforcement, interpolation formatting
  contracts, maximal concat planning and lowering are not implemented;
- `string +=` accepts the direct text operand matrix, but transactional
  fallible commit semantics and in-place proof optimization are not implemented;
- compound assignment does not yet implement every required check and
  evaluation-order guarantee;
- ordinary formatter support exists for many operators but does not yet cover
  every canonical normalization and move token;
- parser recovery for malformed operator expressions is incomplete;
- operator diagnostics are not all registered with stable IDs.

The implemented structured operator checks emit:

```text
S1016  operator.non-orderable-operands
S1017  operator.invalid-shift-count
S1018  operator.signed-left-shift-overflow
S1019  operator.non-comparable-operands
S1021  operator.invalid-membership
S1022  operator.invalid-concat-operand
```

`S1020 operator.string-runtime-concat` is retired and its numeric ID remains
reserved. It must not be emitted or reused.

## Not implemented

The following are not yet implemented completely:

- one generated or shared canonical precedence definition;
- lowering and tooling context for matrix operator `x`;
- complete checked integer arithmetic;
- complete deterministic arithmetic-failure lowering;
- float `%`;
- decimal `%`;
- fixed-array and slice membership Semantic IR and lowering;
- materialization of accepted compile-time string concatenation as one folded
  constant before Semantic IR/lowering;
- active allocation-context resolution for runtime concatenation;
- panic-or-`AllocationError` selection from source `try` context;
- `@noPanic` validation for runtime concatenation;
- interpolation-hole formatting-contract validation;
- canonical maximal `StringConcatPlan` construction and concat/interpolation
  fusion;
- transactional fallible `string +=` semantics;
- canonical `string.Concat` integration and structural `StringBuilder`
  information;
- complete aggregate equality lowering;
- complete detection of user-defined opaque-resource semantics for derived
  struct equality;
- explicit identity or non-comparable type metadata;
- complete operator metadata in Semantic IR;
- complete operator effect metadata;
- complete LSP operator explanations;
- complete operator quick fixes;
- complete operator test matrices;
- a focused diagnostic for reserved `?`.

---

# Purpose

Operators provide concise syntax for common semantic operations.

An operator does not gain meaning solely from its token.

Its resolved meaning depends on:

```text
grammar context
operator category
operand types
operand places
ownership state
contracts
units
target representation
available compiler-known implementations
```

No backend phase may reinterpret an operator differently from Sema.

---

# Scope

This rulebook defines ordinary source operators.

Specialized rulebooks define additional details for:

```text
copy_move.md
    copy and move transfer

ownership.md
    availability and ownership

formatter.md
    canonical printing and normalization

types.md
    primitive and named types

contracts.md
    constrained assignment

collections.md
    indexing, slicing, array equality, and slice behavior

shaped-types.md
    shaped values and matrix multiplication

declarations/spread.md
    spread contexts and ownership

types/units.md
    unit compatibility and unit algebra

references.md
    references and borrowing

raw_pointers.md
    unsafe pointer operations

concurrency_memory_model.txt
    atomics and ordering

memory_model.md
    places, storage, and valid access
```

---

# Definitions

## Operator

An operator is source syntax selecting a compiler-defined semantic operation.

Examples:

```sec
left + right
!enabled
value[index]
object.Member
destination <- source
```

## Operand

An operand is an expression or place consumed, copied, borrowed, inspected, or
mutated by an operator.

## Operator result

An operator result may be:

```text
a value
a place
a reference
a boolean
a range context
a statement effect
no value
```

Assignment operators do not produce expression values in Sec 0.1.

## Operator family

Operators are grouped into:

```text
postfix
prefix
multiplicative
additive
shift
comparison
equality
bitwise
logical
assignment
contextual
grammar-specific
```

---

# General rules

## Compiler-defined meaning

Sec 0.1 does not support arbitrary user-defined operator declarations.

A user cannot declare:

```sec
operator +(left: MyType, right: MyType) MyType
```

Compiler-known and core-known types may support operators through language
rules.

Examples include:

```text
numeric types
named numeric types
unit-bearing types
strings
arrays
enums
unions
structs
vectors
matrices
tensors
references
raw pointers where explicitly allowed
```

This is not arbitrary operator overloading.

## Exact operator resolution

Operator resolution is completed during Sema.

The resolved operation records:

```text
operator kind
left type
right type
result type
conversion plan
copy or move behavior
failure behavior
short-circuit behavior
unit behavior
target requirements
```

## No hidden fallback

When an operator is invalid for its operands, the compiler must not silently:

```text
call an unrelated method
convert through string
allocate
box a value
change copy into move
change move into copy
select a lossy conversion
choose target-specific behavior
```

## No target-dependent source meaning

A target may reject unsupported operations.

A target must not change:

```text
precedence
associativity
evaluation order
overflow semantics
division semantics
comparison semantics
short-circuit behavior
copy or move meaning
```

---

# Canonical precedence

The following table is ordered from highest precedence to lowest precedence.

| Level | Category | Operators and forms | Associativity |
|---:|---|---|---|
| 1 | Member | `.` | left chaining |
| 2 | Postfix | call `()`, index or slice `[]`, struct literal `{}`, spread `...` | left chaining |
| 3 | Prefix | unary `+`, unary `-`, `!`, `~`, and parser-defined prefix forms | right |
| 4 | Multiplicative | `*`, `/`, `%`, contextual `x` | left |
| 5 | Additive | `+`, `-` | left |
| 6 | Shift | `<<`, `>>` | left |
| 7 | Ordered comparison and membership | `<`, `<=`, `>`, `>=`, `in` | non-chainable |
| 8 | Equality | `==`, `!=` | non-chainable |
| 9 | Bitwise AND | `&` | left |
| 10 | Bitwise XOR | `^` | left |
| 11 | Bitwise OR | `|` | left |
| 12 | Logical AND | `&&` | left with short-circuit |
| 13 | Logical OR | `||` | left with short-circuit |

Assignment is not part of expression precedence.

Assignment is statement syntax.

---

# Precedence examples

```sec
a + b * c
```

means:

```sec
a + (b * c)
```

```sec
a << b + c
```

means:

```sec
a << (b + c)
```

```sec
a & b == c
```

means:

```sec
a & (b == c)
```

only if the resulting types are valid.

In ordinary integer code, this will normally be a type error because equality
produces `bool`.

```sec
a == b & c
```

means:

```sec
a == (b & c)
```

```sec
a || b && c
```

means:

```sec
a || (b && c)
```

```sec
left x right + offset
```

means:

```sec
(left x right) + offset
```

---

# Parentheses

Parentheses override precedence.

```sec
(a + b) * c
```

Parentheses may also document intent.

The formatter may remove only provably redundant parentheses according to
`formatter.md`.

Parentheses that preserve readability around mixed boolean or comparison
expressions may remain.

---

# Associativity

## Left-associative binary operators

Unless explicitly stated otherwise, ordinary binary operators associate left.

```sec
a - b - c
```

means:

```sec
(a - b) - c
```

```sec
a / b / c
```

means:

```sec
(a / b) / c
```

```sec
a << b << c
```

means:

```sec
(a << b) << c
```

## Prefix operators

Prefix operators associate right.

```sec
!!value
```

means:

```sec
!(!value)
```

## Non-chainable comparisons

Comparisons are not mathematically chained.

Invalid:

```sec
a < b < c
```

Invalid:

```sec
a == b == c
```

Write:

```sec
a < b && b < c
```

or:

```sec
a == b && b == c
```

The compiler must provide a focused diagnostic.

---

# Evaluation order

## General rule

Sec evaluates expression components strictly from left to right.

This applies to:

```text
binary operands
function arguments
constructor arguments
array literal elements
struct literal fields
index expressions
slice bounds
interpolation components
spread sources
comparison operands
assignment source expressions
```

The compiler may reorder physical instructions only when observable behavior is
unchanged.

Observable behavior includes:

```text
ownership transfer
borrows
destruction
allocation
errors
panic
FFI
volatile access
atomic access
I/O
resource operations
```

## Binary operator

```sec
First() + Second()
```

evaluates:

1. `First()`;
2. `Second()`;
3. addition.

## Function arguments

```sec
Call(First(), Second(), Third())
```

evaluates arguments in source order.

## Index

```sec
values[NextIndex()]
```

evaluates:

1. `values`;
2. `NextIndex()`;
3. bounds validation;
4. access.

## Struct literal

```sec
let value := Record {
    First: CreateFirst(),
    Second: CreateSecond(),
}
```

evaluates fields in source order.

## Spread

A spread source is evaluated exactly once.

Expanded elements are processed in natural source order.

---

# Exact-once place evaluation

An assignment target or compound-assignment target is evaluated exactly once.

Example:

```sec
values[NextIndex()] += NextValue()
```

Conceptual order:

1. evaluate `values`;
2. evaluate `NextIndex()` exactly once;
3. resolve and validate the target place;
4. read the old target value;
5. evaluate `NextValue()`;
6. perform addition;
7. validate overflow, contracts, and other fallible conditions;
8. write the new value.

The compiler must not lower this as:

```sec
values[NextIndex()] = values[NextIndex()] + NextValue()
```

because that would evaluate the target more than once.

---

# Short-circuit evaluation

## Logical AND

```sec
left && right
```

evaluates `left` first.

When `left` is `false`:

- `right` is not evaluated;
- the result is `false`.

When `left` is `true`:

- `right` is evaluated;
- the result is the value of `right`.

## Logical OR

```sec
left || right
```

evaluates `left` first.

When `left` is `true`:

- `right` is not evaluated;
- the result is `true`.

When `left` is `false`:

- `right` is evaluated;
- the result is the value of `right`.

## Path-sensitive effects

A skipped right operand does not:

```text
move a value
create a borrow
allocate
perform I/O
call FFI
access volatile storage
execute an atomic operation
destroy a temporary
produce a runtime error
```

Ownership, borrowing, effects, and diagnostics must follow control flow.

---

# Primary and postfix forms

## Member access

```sec
value.Member
```

Member access has the highest precedence.

The left expression is resolved before the member.

Member access may resolve:

```text
field
method
property
event
nested type
enum member
module member
compiler-known member
```

A member access is a place only when the resolved member and receiver allow it.

## Call

```sec
Function(argument1, argument2)
```

Call syntax may represent:

```text
function call
method call
constructor-like compiler operation
type conversion
generic instantiation followed by call
```

Sema determines the meaning.

The surface syntax alone does not determine whether a form is a function call or
conversion.

## Conversion

```sec
Percent(value)
```

may resolve as a type conversion when `Percent` is a type.

Conversion rules come from `types.md`, named-type rules, units, and contracts.

A conversion is not an ordinary user-overloadable call.

## Index

```sec
values[index]
```

Indexing is postfix syntax.

It may resolve for:

```text
array
slice
list
map
string where explicitly supported
vector
matrix
tensor
compiler-known indexed type
```

Exact indexing semantics belong to the type's rulebook.

## Slice

```sec
values[start..<end]
```

Slicing is contextual postfix syntax.

Slice bounds evaluate left to right.

## Struct literal

```sec
Point {
    X: 1,
    Y: 2,
}
```

The parser distinguishes a struct literal from an ordinary block by grammar
context.

## Spread

```sec
values...
```

Spread is postfix syntax.

Spread is valid only in approved contexts.

Sec 0.1 contexts include:

```text
function arguments
array literals
struct literals
```

Spread is not a general value-producing operator.

---

# Prefix operators

## Unary plus

```sec
+value
```

Unary plus is valid for numeric values.

It preserves the numeric value and resolved type.

It does not convert unsigned to signed or vice versa.

It may be removed by the formatter only where doing so is canonical and does
not affect literal interpretation.

## Unary minus

```sec
-value
```

Unary minus is valid for:

```text
signed integers
floating-point values
decimal values
compatible named numeric types
compatible unit-bearing numeric values
```

Unary minus is invalid for ordinary unsigned runtime values.

A negative untyped literal may be shaped directly into a signed compatible
context.

Signed integer negation is checked.

Negating the minimum representable signed integer causes:

```text
compile-time error
    when known during compilation

deterministic arithmetic failure
    when detected at runtime
```

Floating negation follows floating-point sign semantics.

Decimal negation follows checked decimal representability.

## Logical negation

```sec
!value
```

Operand type must be `bool`.

Result type is `bool`.

No numeric truthiness exists.

Invalid:

```sec
!1
```

## Bitwise complement

```sec
~value
```

Operand must be an integer type.

Result type is the operand type.

The operation flips every value bit in the fixed-width representation.

For a signed integer, interpretation of the result uses the same signed type.

---

# Arithmetic operators

Arithmetic operators include:

```text
+
-
*
/
%
```

They may be available for:

```text
integers
floats
decimals
named numeric types
unit-bearing numeric types
compiler-known shaped values where defined
```

No implicit conversion between unrelated nominal types is introduced merely to
make an operator valid.

---

# Numeric compatibility

Operands must be type-compatible according to numeric conversion rules.

The compiler may shape untyped literals into a compatible operand type.

Examples:

```sec
let value: int32 := 10
let result := value + 2
```

The literal `2` may shape to `int32`.

No implicit conversion occurs between unrelated named types.

```sec
type Meters int
type Seconds int

let distance: Meters := Meters(10)
let time: Seconds := Seconds(2)
```

This is invalid unless a unit or explicit operator rule defines it:

```sec
distance + time
```

---

# Checked integer arithmetic

Integer arithmetic is checked by default.

This includes:

```text
addition
subtraction
multiplication
division overflow
remainder overflow where applicable
unary negation
signed left shift
compound forms of these operations
```

## Compile-time overflow

When overflow is provable at compile time, compilation fails.

## Runtime overflow

When overflow depends on runtime values, the generated program performs a
deterministic arithmetic failure.

Overflow must not:

```text
wrap silently
be undefined behavior
depend on optimization mode
depend on build profile
depend on target instruction behavior
```

## Explicit alternatives

Core may provide explicit operations such as:

```text
WrappingAdd
WrappingSubtract
WrappingMultiply
SaturatingAdd
SaturatingSubtract
CheckedAdd
CheckedSubtract
CheckedMultiply
```

Exact API names belong to core numeric rules.

---

# Integer addition

```sec
left + right
```

Produces the mathematical sum when representable.

Otherwise checked overflow behavior applies.

Unsigned overflow and underflow are checked.

---

# Integer subtraction

```sec
left - right
```

Produces the mathematical difference when representable.

Unsigned underflow is an arithmetic failure.

---

# Integer multiplication

```sec
left * right
```

Produces the mathematical product when representable.

Otherwise checked overflow behavior applies.

---

# Integer division

Integer division truncates toward zero.

Examples:

```text
7 / 3 == 2
-7 / 3 == -2
7 / -3 == -2
-7 / -3 == 2
```

Division by zero causes:

```text
compile-time error
    when divisor is known to be zero

deterministic arithmetic failure
    otherwise
```

For a two's-complement signed type:

```text
minimumValue / -1
```

is checked overflow.

---

# Integer remainder

Integer `%` uses the quotient produced by truncation toward zero.

Definition:

```text
quotient = trunc(left / right)
remainder = left - quotient * right
```

Invariant:

```text
left == quotient * right + remainder
```

Examples:

```text
7 % 3 == 1
-7 % 3 == -1
7 % -3 == 1
-7 % -3 == -1
```

The remainder has the sign of the left operand or is zero.

A zero divisor follows integer division-by-zero rules.

---

# Floating arithmetic

Floating arithmetic follows the selected floating type's IEEE-compatible
semantics unless a target or type rule explicitly rejects an unsupported
operation.

Floating arithmetic does not use integer checked-overflow semantics.

Possible floating results include:

```text
finite value
positive infinity
negative infinity
NaN
positive zero
negative zero
```

The compiler must preserve the language's comparison and remainder rules.

A build profile must not silently switch to unsafe fast-math semantics that
change observable Sec results.

Any future fast-math mode must be explicit and separately specified.

---

# Floating division

Floating division follows floating-point semantics.

Examples conceptually include:

```text
finite / zero
    signed infinity where defined

zero / zero
    NaN

infinity / infinity
    NaN
```

Exact results follow the selected floating format.

---

# Floating remainder

Floating `%` uses truncation-based remainder.

Definition for finite operands:

```text
quotient = trunc(left / right)
remainder = left - quotient * right
```

This is not IEEE nearest-integer `remainder`.

Examples:

```text
7.5 % 2.0 == 1.5
-7.5 % 2.0 == -1.5
7.5 % -2.0 == 1.5
```

Special cases:

```text
finite % finite nonzero
    truncation-based floating remainder

finite % zero
    NaN

infinity % finite
    NaN

finite % infinity
    finite left operand

NaN involved
    NaN
```

The result uses the common resolved floating type.

---

# Decimal arithmetic

Decimal arithmetic follows exact decimal semantics and the selected decimal
type's precision and scale rules.

Operations are checked.

A decimal operation may fail when:

```text
division by zero
result is outside representable range
required scale or precision is not representable
rounding is required but not allowed by the operation
```

Failure must be deterministic and represented according to the decimal and
fallible-expression rules.

Decimal arithmetic must not silently convert to binary float.

---

# Decimal remainder

Decimal `%` uses truncation-based remainder.

Definition:

```text
quotient = trunc(left / right)
remainder = left - quotient * right
```

A decimal remainder is exact when representable.

A zero divisor or unrepresentable result causes deterministic arithmetic
failure.

The result type follows the resolved decimal compatibility rules.

---

# String concatenation

Runtime string concatenation with `+` is supported in Sec 0.1.

## Direct operand matrix

The only direct operand types are the built-in, non-nominal types `string`,
`char`, and `rune`. Every valid pair produces `string`:

```text
string + string -> string
string + char   -> string
char   + string -> string
string + rune   -> string
rune   + string -> string
char   + char   -> string
rune   + rune   -> string
char   + rune   -> string
rune   + char   -> string
```

A `char` or `rune` already represents one textual element and does not require
an explicit conversion.

## No hidden general conversion

The operator does not convert arbitrary operands to string. Numbers, `bool`,
enums, structs, unions, user-defined nominal types, arrays, slices,
collections, interfaces, and all other non-text values require explicit
conversion:

```sec
let valid := "Count: " + count.ToString()
let invalid := "Count: " + count
```

The compiler must not silently insert `ToString()`.

## Interpolation and formatting

Interpolation is an explicit formatting context and is semantically distinct
from direct concatenation:

```sec
let text := $"Count: {count}"
```

An interpolation hole is valid when its value has a canonical formatting
contract. Initially, a user-defined type normally supplies that contract through
`ToString()`. This does not make direct `"value: " + value` valid; direct
concatenation still requires `value.ToString()`.

Mixed interpolation and concatenation are valid. Interpolation formatting,
`value.ToString()`, and a future `value.ToString(format)` must share one coherent
formatting model. This rule does not lock the exact format syntax, format type,
or locale model.

## Compile-time concatenation

When the complete concatenation is compile-time evaluable, it folds to one
string constant with:

```text
no runtime allocation
no allocation context requirement
no try
no allocation-failure panic
no StringBuilder recommendation
```

## Runtime allocation and failure policy

Runtime concatenation uses the active allocation context. Missing usable
allocation context is a compile-time error.

Without `try`, runtime concatenation produces `string` on success and accepts a
deterministic allocation panic:

```sec
s = "Hello " + name
```

With `try`, allocation failure is propagated or locally handled as
`AllocationError`, while the successful value remains `string`:

```sec
let result := try ("Hello " + name)
```

| Form | May allocate | May panic from result allocation |
|---|---:|---:|
| Fully compile-time-folded concatenation | No | No |
| Runtime concatenation without `try` | Yes | Yes |
| Runtime concatenation with `try` | Yes | No |
| Runtime concatenation without allocation context | Compile-time error | Not applicable |

Optimization may eliminate allocation or prove success, but must not change the
source-selected failure policy.

## `@noPanic` and `try` scope

Runtime concatenation without `try` is incompatible with `@noPanic`. A valid
`try` form may be used when its `AllocationError` flow is handled or propagated.

`try` covers the concatenation's explicit fallible flow and nested operations
whose error flow is explicitly part of the expression. It does not catch an
arbitrary panic from operand evaluation or a user-defined `ToString()` call.
Those operations retain their declared effects and execute exactly once.

## Maximal concatenation plan

A maximal concatenation and interpolation chain becomes one semantic
`StringConcatPlan`, not nested binary allocations. Conceptual segments include:

```text
ConstantStringSegment
StringSegment
CharSegment
RuneSegment
BuiltinFormattedSegment
MaterializedStringSegment
InterpolationSegment
```

Compatible parenthesized text expressions may remain one plan. A separate
binding creates an observable semantic result; fusion across that boundary is
only an optimization after ownership, lifetime, and effect proof.

Every segment evaluates left to right and exactly once. The plan may then
measure segment lengths, calculate the total with checked arithmetic, allocate
canonical result storage once, and write segments in source order. Static
impossibility is a compile-time error; dynamic length failure follows the
selected try-or-panic policy and never wraps silently.

The canonical result is allocated at most once. User-defined `ToString()` calls
may allocate separate temporary strings; compiler-known formatting may be fused
without a temporary.

## Compound concatenation assignment

`string +=` uses the same direct operand matrix and failure-policy choice:

```sec
s += part
try s += part
```

It is transactional: evaluate the destination place once, read the old value,
evaluate the appended segment once, construct and validate the new value, then
commit. A failed `try` leaves the destination unchanged. In-place append is
permitted only after proof of unique ownership, sufficient capacity, no
conflicting alias, stable storage, and compatible allocation context.

## `string.Concat`

`string.Concat` is an explicit core API that shares segment order, accepted
text operand categories, allocation context, try-or-panic selection, checked
length calculation, and canonical concat Semantic IR with `+`. Its exact
overload set belongs to the string/core rulebook; unrestricted variadics are not
defined here.

## `StringBuilder`

`StringBuilder` is preferred for repeated incremental construction such as
loops, serialization, code generation, and unknown segment counts. It is not
required for one finite maximal concatenation expression.

Repeated reconstruction may produce configurable information based on control
flow and mutation structure. The compiler must not inspect string content to
infer intent. Do not report one maximal expression, compile-time concatenation,
one isolated `+=`, a chain already planned as one allocation, proven in-place
reuse, or construction below the configured threshold.

---

# Unit-bearing arithmetic

Unit-bearing numeric values use unit algebra.

Examples:

```sec
distance + otherDistance
distance - otherDistance
speed * time
distance / time
```

General rules:

- ordinary numeric operator validity is checked first;
- `+`, `-`, and comparison require compatible dimensions, known Kind,
  transform, point/difference role, Origin where applicable, and an exact valid
  conversion plan;
- different known Kinds are not implicitly combined merely because dimensions
  match;
- ordinary linear `*` and `/` produce structural unit expressions and do not
  invent a named result or Kind;
- `%` requires compatible operands and preserves the left operand's unit;
- point algebra is `P-P -> D`, `P+D -> P`, `P-D -> P`, `D+D -> D`, and
  `D-D -> D`; `P+P` and ordinary point multiplication/division are invalid;
- logarithmic quantities do not silently use ordinary linear arithmetic;
- implicit conversion must be exactly representable without hidden rounding,
  truncation, overflow, or precision loss;
- structural currency results remain valid but may produce warning diagnostics;
- contracts and representability remain checked.

The complete unit algebra belongs to `rules/types/units.md`.

---

# Shaped arithmetic

Shaped values may define compiler-known arithmetic.

Ordinary `*` remains scalar or elementwise multiplication according to shaped
type rules.

Contextual `x` is matrix multiplication.

No arbitrary user-defined shaped operator overloading exists in Sec 0.1.

---

# Matrix multiplication operator `x`

## Lexical status

`x` is lexed as an identifier.

It becomes matrix multiplication only in a parser-confirmed infix expression
context with compatible shaped operands.

This permits ordinary identifiers named `x`.

```sec
let x := 10
```

and matrix multiplication:

```sec
let result := left x right
```

## Precedence

`x` has the same precedence as:

```text
*
/
%
```

It associates left.

```sec
a x b x c
```

means:

```sec
(a x b) x c
```

when shapes are valid.

## Sec 0.1 operand forms

Initial supported forms are defined by shaped-type rules and include:

```text
matrix x matrix
matrix x vector
```

Other forms require explicit rules.

## Shape validation

The inner dimensions must be compatible.

Example:

```text
Matrix[M, K] x Matrix[K, N]
    -> Matrix[M, N]
```

Shape mismatch is a compile-time error when dimensions are known.

Runtime-shaped forms require deterministic validation.

## Formatter

Canonical spacing:

```sec
left x right
```

The formatter must distinguish infix `x` from an identifier.

---

# Bitwise operators

Bitwise operators are:

```text
&
|
^
~
```

Operands must be integer types.

Binary operands must be compatible.

Result type is the resolved integer operand type.

Bitwise operations use the fixed-width bit representation of the type.

They do not perform boolean operations.

Invalid:

```sec
true & false
```

Use:

```sec
true && false
```

---

# Bitwise AND

```sec
left & right
```

Each result bit is one only when both corresponding operand bits are one.

---

# Bitwise OR

```sec
left | right
```

Each result bit is one when either corresponding operand bit is one.

---

# Bitwise XOR

```sec
left ^ right
```

Each result bit is one when corresponding operand bits differ.

---

# Shift operators

Shift operators are:

```text
<<
>>
```

Both operands must be integers.

The result type is the left operand's type.

---

# Shift count

A shift count is valid only when:

```text
0 <= count < bit width of left operand
```

Invalid counts include:

```text
negative count
count equal to bit width
count greater than bit width
```

When invalidity is compile-time known, compilation fails.

Otherwise the program performs a deterministic arithmetic failure before the
target shift instruction.

Sec does not inherit target-specific count masking.

Example:

```text
32-bit value << 35
```

does not silently become:

```text
value << 3
```

---

# Left shift

## Unsigned left shift

Unsigned `<<` performs a fixed-width bit shift.

Bits shifted beyond the representation are discarded.

Zeros enter from the right.

This is defined bit behavior, not checked arithmetic overflow.

## Signed left shift

Signed `<<` is checked.

It is valid only when the mathematical shifted result is representable in the
signed left operand type.

Otherwise:

```text
compile-time error
    when known

deterministic arithmetic failure
    at runtime
```

The compiler may lower directly to a target shift instruction after proving or
checking validity.

---

# Right shift

## Unsigned right shift

Unsigned `>>` is logical right shift.

Zeros enter from the left.

## Signed right shift

Signed `>>` is arithmetic right shift.

The sign bit is propagated.

This behavior is defined by Sec and does not depend on the source target
language or backend default.

---

# Equality

Equality operators are:

```text
==
!=
```

Result type is `bool`.

Equality requires compatible equality-comparable operands.

No chained equality exists.

---

# Boolean equality

```sec
left == right
left != right
```

is valid for `bool`.

---

# Numeric equality

Compatible numeric values may be compared.

Untyped literals may shape into the compatible numeric type.

Unrelated nominal types do not become comparable merely because they share a
representation.

---

# Floating equality

Floating equality follows these rules:

```text
NaN == NaN
    false

NaN != NaN
    true

+0.0 == -0.0
    true

+0.0 != -0.0
    false
```

A comparison involving NaN follows IEEE-style unordered semantics.

A future total-order operation for sorting is explicit and not `==`.

---

# Character and rune equality

`char` compares with `char`.

`rune` compares with `rune`.

No implicit integer comparison exists.

Valid:

```sec
if ch == 0r {
}
```

Valid:

```sec
if character == 0t {
}
```

Invalid:

```sec
if ch == 0 {
}
```

A character literal may be context-shaped according to literal rules.

Example:

```sec
if ch == '$' {
}
```

when `ch` is a `rune`.

A `char` variable and `rune` variable do not implicitly compare with one
another.

Explicit conversion is required when valid.

---

# String equality

String equality compares exact immutable string content.

It does not compare:

```text
storage address
allocation identity
descriptor identity
padding
reference identity
```

No locale, normalization, or case folding is performed.

---

# Enum equality

Values of the same enum type support:

```text
==
!=
```

Aliases compare by resolved enum value.

Different enum types do not compare implicitly even when their underlying
integer representation is the same.

Enums do not derive ordered comparison in Sec 0.1.

---

# Array equality

Fixed arrays support structural equality when their element type is
equality-comparable.

Comparison proceeds element by element in index order.

It short-circuits at the first unequal element.

Array equality does not compare padding or storage address.

---

# Slice equality

Slices do not support `==` or `!=` as content comparison in Sec 0.1.

A slice is a view.

Content comparison uses an explicit operation.

Reference or descriptor identity comparison is not exposed through ordinary
slice equality.

---

# Struct equality

A struct derives structural equality when:

- every stored field is equality-comparable;
- the type has no explicit identity semantics;
- the type has no opaque-resource semantics;
- the type is not explicitly non-comparable;
- equality does not violate a custom invariant.

A normal `impl` block does not disable derived equality.

Example:

```sec
type Point struct {
    X: int,
    Y: int,
}

impl Point {
    fn LengthSquared() int {
        return self.X * self.X + self.Y * self.Y
    }
}
```

`Point` may still support:

```sec
first == second
```

Derived comparison:

- processes stored fields in declaration order;
- uses each field's normal equality;
- short-circuits on the first unequal field;
- does not read padding;
- does not compare storage address;
- does not compare methods, properties, or type metadata.

Types that normally do not derive equality include:

```text
File
Socket
Mutex
atomic storage object
DeviceHandle
foreign opaque object
types with external unique identity
types explicitly marked non-comparable
```

A custom `free` operation is a strong reason to require an explicit equality
decision.

The exact declaration syntax for identity or non-comparable semantics belongs to
type and attribute rules.

---

# Union equality

A union supports equality only when its union rules guarantee a safe comparison.

Typical derived behavior:

1. compare active tags;
2. if tags differ, result is unequal;
3. compare the active payload using payload equality.

All possible compared payloads must be equality-comparable.

Inactive payload storage and padding are not compared.

---

# Reference equality

Safe references do not expose general referent-address equality in Sec 0.1
unless a specialized reference rule explicitly provides it.

Content equality compares dereferenced values explicitly where valid.

This avoids confusing:

```text
same storage
same value
same owner
same resource
```

`reference_model.md` is the specialized reference rule for Sec 0.1. It defines
ordinary equality for compatible safe references as comparison of live storage
identity and referenced location. This does not add equality for `ref T[]` or
`ref mut T[]`, which are slice views under the array/slice rules.

Other identity queries remain explicit APIs rather than additional ordinary
operators.

---

# Raw pointer equality

`RawPtr[T]` may support address-value equality in unsafe or low-level contexts
according to `raw_pointers.md`.

Such equality compares address representation only.

It does not prove:

```text
same active object
same provenance
same allocation
same lifetime
safe dereference
```

---

# Interface equality

Direct equality of general interface values is not part of Sec 0.1.

The compiler must first have a complete model for:

```text
borrowed erased values
owned erased values
dynamic type identity
dynamic equality
move-only payloads
opaque resources
```

Concrete values may be compared before erasure.

---

# Ordered comparison

Ordered comparison operators are:

```text
<
<=
>
>=
```

Result type is `bool`.

They are valid only for approved ordered types.

No chained ordered comparison exists.

---

# Numeric ordering

Compatible numeric values support ordered comparison.

Integer and decimal ordering use mathematical value order.

Floating ordering follows floating-point comparison rules.

---

# Floating ordering

For any NaN operand:

```text
left < right
left <= right
left > right
left >= right
```

all evaluate to `false`.

Positive and negative zero compare equal and neither is ordered before the
other through ordinary comparison.

A deterministic total order for sorting NaN payloads, signed zero, and bit
patterns must use an explicit operation.

---

# Character ordering

`char` values order by Unicode scalar value within the valid `char` domain.

No locale or alphabetic collation is applied.

---

# Rune ordering

`rune` values order by Unicode scalar value.

No locale, normalization, or case folding is applied.

---

# String ordering

Strings order lexicographically by Unicode scalar sequence.

Comparison proceeds from the first scalar value.

At the first difference, scalar-value ordering determines the result.

When one string is an exact prefix of the other, the shorter string orders
first.

No operation is performed for:

```text
locale collation
Swedish alphabetic ordering
case folding
Unicode normalization
natural-number ordering
```

Those require explicit library operations.

---

# Types without ordering

Ordinary ordered comparison is not provided for:

```text
bool
enum
struct
union
array
slice
map
set
list
reference
raw pointer
interface
resource handle
```

unless a specialized canonical rule explicitly adds it.

---

# Membership operator `in`

`in` has two different grammar roles.

They must not be conflated.

---

# Membership expression

Expression form:

```sec
value in collection
```

returns `bool`.

Sec 0.1 supports membership for:

```text
contextual range
fixed array
slice
```

It does not use arbitrary method lookup.

---

# Range membership

```sec
value in lower..<upper
```

or:

```sec
value in lower..upper
```

uses the range's inclusive or exclusive bound semantics.

Bounds evaluate left to right.

The value evaluates before the right range expression.

Range membership uses compatible ordered values.

---

# Fixed-array membership

```sec
value in values
```

when `values` is a fixed array:

- evaluates `value` first;
- evaluates the array expression once;
- examines elements from index zero upward;
- uses element equality;
- stops at the first match;
- returns `true` on match;
- returns `false` after all elements differ;
- performs no allocation.

The array element type must be equality-comparable.

The left value must be compatible with the element type.

---

# Slice membership

Slice membership follows the same content semantics as array membership.

```sec
value in view
```

- evaluates `value` first;
- evaluates the slice expression once;
- reads elements left to right;
- stops at first match;
- performs no allocation;
- does not structurally mutate the slice or backing storage.

The slice and references must remain valid during the operation.

---

# Unsupported membership in Sec 0.1

The `in` operator is not defined for:

```text
list
set
map
string
iterator
generator
arbitrary user type
```

Use explicit APIs.

Examples:

```sec
values.Contains(item)
map.ContainsKey(key)
text.Contains(substring)
```

This may be extended in a future language version.

---

# `for ... in ...`

Iteration syntax:

```sec
for value in values {
}
```

is not the membership operator.

It is part of the `for` grammar.

It:

- obtains an iteration source;
- binds each produced element;
- follows iterator ownership and borrow rules;
- does not return `bool`;
- does not perform a membership search.

The parser distinguishes the forms by grammar context.

---

# Ranges

Ranges are contextual syntax in Sec 0.1.

Valid uses include:

```sec
for i in 0..<10 {
}

if value in 0..100 {
}

let view := values[2..<8]
```

A range is not a general first-class runtime value in Sec 0.1.

Invalid:

```sec
let range := 0..<10
```

No general `Range[T]` variable type is implied.

---

# Inclusive range

```sec
start..end
```

includes both endpoints where the context permits.

---

# Exclusive upper range

```sec
start..<end
```

includes `start` and excludes `end`.

This is the canonical half-open form for indexing and iteration.

---

# Range operands

Range operand types and step behavior belong to iteration, slicing, and range
rules.

The parser must not accept chained ranges as one expression.

Invalid:

```sec
a..<b..<c
```

---

# Logical operators

Logical operators are:

```text
&&
||
!
```

All operands must be `bool`.

Sec does not use truthy or falsy conversions.

---

# Assignment statements

Assignment is statement syntax.

It does not produce a value.

Invalid:

```sec
let result := destination = source
```

Invalid:

```sec
a = b = c
```

Assignments do not associate.

---

# Ordinary declaration initialization

```sec
let destination := source
```

`:=` is ordinary inferred initialization.

From an existing reusable place, it requires copyability.

It never silently moves a move-only value.

Fresh temporaries may initialize directly.

---

# Move declaration initialization

```sec
let destination :<- source
```

`:<-` explicitly moves from an existing source place and infers destination
type.

The source becomes unavailable.

---

# Typed ordinary initialization

```sec
let destination: Type := source
```

Ordinary copy or direct construction semantics apply.

---

# Typed move initialization

```sec
let destination: Type <- source
```

The explicit type uses `:`.

The transfer operator is `<-`.

---

# Ordinary assignment

```sec
destination = source
```

This is ordinary assignment, replacement, or reinitialization.

From an existing reusable source place, copyability is required.

It never silently moves.

---

# Move assignment

```sec
destination <- source
```

This explicitly transfers the source value or ownership responsibility.

The source becomes unavailable.

---

# Assignment evaluation order

For:

```sec
destination = source
```

conceptual order:

1. evaluate destination place exactly once;
2. evaluate source expression;
3. validate conversion, contracts, ownership, and borrows;
4. preserve old destination until commit is safe;
5. destroy old destination when replacement requires it;
6. install the new value.

Move assignment additionally marks source unavailable only after valid commit.

---

# Compound assignment

Compound assignment operators are:

```text
+=
-=
*=
/=
%=
&=
|=
^=
<<=
>>=
```

Conceptually:

```sec
destination op= source
```

means:

```text
read destination once
evaluate source once
apply operator
validate result
write destination once
```

It is not defined as textual rewriting.

---

# Compound assignment target

The target must be a writable mutable place.

Possible targets include:

```text
mutable local
mutable field
mutable property with setter
mutable indexed place
mutable dereferenced reference
compiler-known writable location
```

Exact validity depends on the target type and place rules.

---

# Compound assignment and contracts

When the destination type has a fallible contract, compound assignment requires
the canonical `try` form.

Example:

```sec
try percent += Percent(i) {
    Err(error) => {
        discard error
    }
}
```

The old destination remains valid until the new result passes validation.

---

# Compound arithmetic failure

Arithmetic failure from a compound operator follows the underlying operator.

Examples:

```text
+=
    checked addition

/=
    division-by-zero and overflow rules

%=
    remainder rules

<<=
    shift-count and signed-overflow rules
```

No destination mutation commits before successful validation.

---

# Increment and decrement aliases

Sec accepts statement-only:

```sec
value++
value--
```

They have no expression result.

The formatter normalizes:

```sec
value++
```

to:

```sec
value += 1
```

and:

```sec
value--
```

to:

```sec
value -= 1
```

Invalid:

```sec
let previous := value++
```

The resulting compound assignment still follows:

```text
mutability
overflow
contracts
try
ownership
```

---

# Self-assignment

Ordinary copy self-assignment:

```sec
value = value
```

may be accepted for copyable values and optimized to no operation.

The compiler may emit an advisory diagnostic.

Move self-assignment is invalid:

```sec
value <- value
```

---

# Overlapping places

Assignment and move operations must validate place overlap.

Potentially invalid:

```sec
object.Field <- object
```

when source and destination overlap incompatibly.

The canonical place model from `memory_model.md` determines:

```text
same
disjoint
contains
contained by
potentially overlapping
unknown
```

---

# `ref`, `try`, `await`, and `spawn`

These are parser-defined prefix expression forms or statement forms.

They are not ordinary symbolic operators available for user overloading.

Their detailed semantics belong to:

```text
references and borrowing rules
error and try rules
tasks and await rules
spawn rules
```

They occupy prefix parsing positions where applicable.

---

# Question mark `?`

`?` is lexed and reserved.

It has no language meaning in Sec 0.1.

The parser should emit a focused diagnostic:

```text
`?` is reserved and has no meaning in Sec 0.1
```

Its future use remains open.

Possible future designs may include conditionals or another language feature.

Sec 0.1 does not assign Rust-style error propagation or any other meaning.

---

# Unsupported operators

Sec 0.1 does not define:

```text
ternary `?:`
null-coalescing `??`
optional access `?.`
exponentiation `**`
pipeline operators
spaceship comparison
identity comparison
user-declared operators
comma expression
assignment expression
```

A future language version may define new syntax through a separate rule.

---

# Operator failure classes

An operator may be:

```text
infallible
compile-time rejected
runtime checked
fallible through Result or try
short-circuiting
target-restricted
unsafe
```

The resolved operator must record its class.

---

# Deterministic arithmetic failure

Operations such as checked integer overflow and runtime integer division by zero
require a deterministic arithmetic-failure path.

The exact representation may depend on the surrounding expression and profile.

It must not become:

```text
undefined behavior
silent wrapping
random target trap semantics
optimizer-dependent behavior
```

A future arithmetic-error rulebook may define:

```text
Result integration
panic integration
checked expression syntax
target trap lowering
no-runtime lowering
```

Until then, the compiler must preserve a distinguishable failure edge.

---

# Constant folding

The compiler may evaluate operators at compile time when operands are constant.

Constant folding must use Sec semantics.

It must not use host-language behavior when that differs.

Examples include:

```text
checked overflow
truncation-toward-zero division
float NaN comparison
unit conversion
string literal concatenation
shift-count validation
```

A constant expression that would deterministically fail is a compile-time error.

---

# Untyped literals

Untyped literals may be shaped by operator context.

Examples:

```sec
let value: int32 := 10
let result := value + 1
```

The literal may become `int32`.

Literal shaping must not introduce:

```text
overflow
lossy conversion
nominal type confusion
char/rune integer comparison
unit mismatch
```

---

# Named types

A named type retains nominal identity.

An operator may be available when:

- the named type's canonical rule permits it;
- operands are the same compatible named type;
- a literal shapes directly into that type;
- an explicit conversion has occurred;
- a unit rule defines compatible algebra.

Underlying representation alone does not grant every base-type operator.

---

# Contracts

Operator results assigned to constrained types must satisfy contracts.

A contract may be:

```text
compile-time proven
runtime checked
fallible
```

Operators do not bypass contracts.

A same-type move preserves an already valid value.

Arithmetic producing a new value requires normal validation.

---

# Enums

Enums support:

```text
==
!=
```

according to enum rules.

Enums do not automatically support:

```text
+
-
*
/
%
<
<=
>
>=
bitwise operators
shifts
```

even when the underlying representation is integer.

Any future flags-enum model requires a separate rule.

---

# Registers

Register expressions may support compiler-known bitwise and numeric operations on
snapshots.

Access to addressed register storage follows:

```text
volatile rules
exact access-width rules
reserved-bit rules
fixed-address rules
target rules
```

An operator on a local snapshot is not itself a hardware write.

Compound assignment to a register-backed property may lower to a defined
read-modify-write operation only when register rules permit it.

---

# Raw pointers

Raw pointer arithmetic, comparison, and difference are not granted by ordinary
numeric operator rules.

Any supported raw-pointer operator must be:

```text
explicitly defined
unsafe where required
provenance-aware
bounds-aware where known
target-representable
```

Address-value equality may be permitted by `raw_pointers.md`.

Ordered raw-pointer comparison is not part of ordinary Sec 0.1 operators.

---

# Atomics

Ordinary operators on atomic storage are not automatically atomic
read-modify-write operations.

Atomic types expose compiler-known methods or explicit atomic operations with
memory ordering.

The source:

```sec
atomicValue += 1
```

must not silently become an atomic fetch-add unless an atomic rule explicitly
defines that syntax.

---

# Volatile values

Ordinary arithmetic on a local value loaded from volatile storage is ordinary
arithmetic.

Accessing volatile storage must remain an explicit volatile load or store in
Semantic IR.

The compiler must not duplicate or eliminate volatile accesses while applying
operator transformations.

Compound assignment to volatile storage requires one defined read and one
defined write unless a register rule says otherwise.

---

# Formatter rules

Canonical spacing:

```sec
left + right
left - right
left * right
left / right
left % right
left x right
left << right
left >> right
left < right
left <= right
left == right
left != right
left & right
left ^ right
left | right
left && right
left || right
destination = source
destination <- source
let value := source
let value :<- source
```

Prefix operators have no following space:

```sec
-value
+value
!value
~value
```

Member access has no surrounding spaces:

```sec
value.Member
```

Calls and indexes have no preceding space:

```sec
Call(value)
values[index]
```

---

# Formatter normalization

Ordinary formatter may normalize parser-confirmed:

```text
func -> fn
x++ -> x += 1
x-- -> x -= 1
```

It must preserve:

```text
:= versus :<-
= versus <-
```

It must not infer semantic move syntax.

It must recognize contextual `x` before formatting it as an infix operator.

An identifier named `x` remains an identifier.

---

# Parser recovery

The parser should recover from operator errors without discarding the complete
surrounding expression.

Examples:

```text
missing right operand
missing left operand
duplicated operator
invalid chained comparison
assignment in expression context
reserved question mark
missing closing delimiter
```

Recovery nodes should preserve:

```text
operator token
known operand
missing operand position
expected category
source range
```

This supports LSP diagnostics and fixes.

---

# Diagnostics

Operator diagnostics require stable IDs.

Suggested rules:

```text
parser.missing-operator-operand
parser.invalid-assignment-expression
parser.chained-comparison
parser.reserved-question-mark

sema.operator-undefined
sema.operator-type-mismatch
sema.operator-nominal-type-mismatch
sema.operator-unit-mismatch
sema.operator-non-comparable
sema.operator-non-orderable
operator.invalid-concat-operand
operator.concat-missing-allocation-context
effect.concat-may-panic
operator.concat-length-overflow
performance.string-builder-recommended
sema.operator-invalid-membership
sema.operator-invalid-range-value
sema.operator-invalid-shift-count
sema.operator-shift-overflow
sema.operator-integer-overflow
sema.operator-division-by-zero
sema.operator-remainder-by-zero
sema.operator-invalid-compound-target
sema.operator-move-self-assignment
sema.operator-overlapping-move
```

The current registry assigns these IDs to the implemented subset:

```text
S1016  operator.non-orderable-operands
S1017  operator.invalid-shift-count
S1018  operator.signed-left-shift-overflow
S1019  operator.non-comparable-operands
S1021  operator.invalid-membership
S1022  operator.invalid-concat-operand
```

Retired and permanently reserved:

```text
S1020  operator.string-runtime-concat
```

Stable IDs for the remaining concat categories must be assigned through the
diagnostic registry when their checks are implemented.

Safety errors are mandatory.

Performance findings may be advisory.

---

# Diagnostic examples

## Invalid concatenation operand

```text
error[S1022]: `int` cannot be concatenated directly with `string`
string concatenation accepts string, char, and rune
help: use `value.ToString()` or interpolation when a formatting contract exists
```

## Invalid rune comparison

```text
error[S....]: rune cannot be compared with int
help: use `0r` for rune zero or convert explicitly
```

## Invalid char comparison

```text
error[S....]: char cannot be compared with int
help: use `0t` for char zero or convert explicitly
```

## Chained comparison

```text
error[P....]: comparisons cannot be chained
help: write `a < b && b < c`
```

## Invalid shift count

```text
error[S....]: shift count 32 is outside the valid range 0..<32
```

## Invalid membership

```text
error[S....]: `in` supports ranges, fixed arrays, and slices in Sec 0.1
```

## Missing concatenation allocation context

```text
error[S....]: runtime string concatenation requires an allocation context
```

## Concatenation in `@noPanic`

```text
error[S....]: runtime concatenation without `try` may panic on allocation failure
help: use the valid `try` form and handle or propagate AllocationError
```

---

# LSP integration

The LSP should expose at an operator:

```text
resolved operation
operand types
result type
precedence
associativity
evaluation order
short-circuit behavior
overflow behavior
failure behavior
unit transformation
copy or move behavior
target support
```

Example hover:

```text
Operator: signed integer addition
Type: int32
Overflow: checked
Evaluation: left operand, then right operand
Result: int32
```

Example for `&&`:

```text
Operator: lazy logical AND
Right operand executes only when left is true
```

Example for `x`:

```text
Operator: matrix multiplication
Left: Matrix[4, 3, float32]
Right: Matrix[3, 2, float32]
Result: Matrix[4, 2, float32]
```

Example for runtime string `+`:

```text
Operator: string concatenation
Result: string
May allocate: yes
Allocation context: current arena
Failure policy: panic or AllocationError through try
Concat plan: maximal chain
```

---

# Code actions

Possible operator code actions include:

```text
split chained comparison
replace invalid rune integer literal with `0r`
replace invalid char integer literal with `0t`
insert explicit conversion
insert an explicit `.ToString()` for an invalid concat operand
convert an invalid concat expression to interpolation
replace repeated reconstruction with `StringBuilder`
replace unsupported membership with `.Contains(...)`
add required `try`
replace invalid copy assignment with explicit move
parenthesize expression
```

A fix must be machine-applicable only when intent is unique.

---

# Semantic IR

Every resolved operator must lower to an explicit semantic operation.

Conceptual operations include:

```text
UnaryPlus
NegateChecked
LogicalNot
BitwiseNot

AddChecked
SubtractChecked
MultiplyChecked
DivideIntegerChecked
RemainderIntegerChecked

AddFloat
SubtractFloat
MultiplyFloat
DivideFloat
RemainderFloat

AddDecimalChecked
SubtractDecimalChecked
MultiplyDecimalChecked
DivideDecimalChecked
RemainderDecimalChecked

MatrixMultiply

ShiftLeftUnsigned
ShiftLeftSignedChecked
ShiftRightLogical
ShiftRightArithmetic

CompareEqual
CompareNotEqual
CompareOrdered
CompareFloatOrdered
CompareStringLexical
CompareStructStructural

LogicalAndShortCircuit
LogicalOrShortCircuit

RangeMembership
ArrayMembership
SliceMembership

CopyInitialize
MoveInitialize
CopyAssign
MoveAssign
CompoundAssign

Index
Slice
MemberAccess
Call
Conversion
Spread
StringConcatPlan
```

Exact operation names may differ.

The semantic distinctions must remain explicit.

---

# Semantic IR operator metadata

Where relevant, record:

```text
operator token
operator kind
left type
right type
result type
source locations
evaluation order
short-circuit edges
overflow mode
division mode
remainder mode
shift mode
comparison mode
unit algebra
contract validation
copy or move behavior
target requirements
failure edge
ordered concat segments
concat segment semantic kinds
active allocation context
concat failure policy
allocation and panic effects
checked total-length behavior
compile-time-folded status
temporary materialization requirements
```

---

# Semantic IR verification

Before MLIR lowering:

- every operator has resolved operand types;
- every operator has one resolved semantic meaning;
- every comparison is valid;
- every assignment target is valid;
- every move source is available;
- every compound target is evaluated once;
- every checked arithmetic operation has a failure edge or proof;
- every short-circuit expression has explicit control flow;
- every shift count is proven or checked;
- every concat segment type is valid;
- every interpolation hole has a formatting contract;
- every runtime concat allocation context and failure policy is explicit;
- every concat segment evaluates left to right and exactly once;
- total concat length arithmetic is checked;
- every runtime concatenation is represented by one maximal
  `StringConcatPlan`, not unresolved allocating binary `+` operations;
- `@noPanic` constraints are satisfied;
- compound concat assignment commits only after success;
- every membership source is supported;
- every range remains in a valid contextual position;
- contextual `x` has a shaped operation;
- no arbitrary operator overload remains unresolved.

Failure is an internal compiler error after successful Sema.

---

# MLIR lowering

MLIR may use:

```text
arith
math
scf
cf
memref
tensor
linalg
LLVM dialect
target-specific dialects
```

The selected lowering must preserve Sec semantics.

Examples:

```text
checked integer add
    overflow-producing operation plus explicit failure control flow

short-circuit &&
    control-flow blocks and merged boolean result

matrix x
    shaped contraction or equivalent verified lowering

array membership
    left-to-right loop with early exit

string lexical comparison
    compiler-known scalar-sequence comparison

volatile compound assignment
    one volatile read, operation, one volatile write
```

---

# LLVM and backend requirements

Backends must not rely on undefined target-language behavior for Sec checked
operations.

They must preserve:

```text
strict left-to-right observable evaluation
short-circuit control flow
checked overflow
division-by-zero handling
shift validation
signed and unsigned right-shift distinction
float NaN comparison rules
copy and move semantics
volatile and atomic effects
```

---

# Float comparison lowering requirement

Ordinary float `!=` must be true when either operand is NaN.

A backend predicate equivalent to ordered-not-equal is insufficient.

The lowering must implement unordered-not-equal semantics for Sec `!=`.

Ordered comparisons remain false with NaN.

---

# Optimization

The compiler may optimize operators when results and observable behavior remain
equivalent.

Allowed examples:

```text
constant folding
strength reduction
copy elision
common subexpression elimination for pure expressions
vectorization
matrix lowering
short-circuit simplification
bounds-check elimination
overflow-check elimination after proof
shift-check elimination after proof
constant concat-segment merging
maximal compatible concat-tree flattening
one canonical concat-result allocation
compiler-known formatting fusion
proven unique-backing-storage reuse
```

The compiler must not optimize away:

```text
volatile access
atomic operation
FFI effect
resource effect
required failure edge
destruction
ownership transfer
observable allocation
source-selected concatenation failure policy
left-to-right exact-once concat evaluation
transactional string `+=` commit
```

---

# Constant-expression rules

Operators usable in constant expressions must:

```text
be deterministic
have compile-time-known operands
not require runtime allocation
not perform I/O
not access volatile memory
not depend on runtime target state
```

Compile-time string literal concatenation is allowed.

It folds to one string constant and requires no runtime allocation context,
`try`, or allocation-failure policy.

Compile-time arithmetic uses Sec checked semantics.

---

# Testing

## String concatenation tests

Test all nine ordered pairs of `string`, `char`, and `rune`; every result must
be `string`. Reject direct integer, float, decimal, bool, enum, struct, union,
array, slice, collection, interface, and user-defined nominal operands with
`S1022`, both operand types, the accepted text categories, and explicit
conversion/interpolation help.

Test compile-time folding, interpolation formatting contracts, mixed chains,
left-to-right exact-once evaluation, active allocation contexts, panic and
`try` policies, `@noPanic`, checked total length, one maximal concat plan, and
at-most-once canonical result allocation.

For `string +=`, test exact-once destination and source evaluation,
transactional failure, unchanged destination after failed `try`, and semantic
equivalence under proven in-place optimization. Test that structural
`StringBuilder` information appears only for repeated reconstruction and never
for one isolated operation or one maximal chain.

Verify that valid runtime concatenation never emits retired `S1020`, that the
ID is not reused, and that every replacement diagnostic has a stable registry
entry before it is emitted.

## Lexer tests

Test every operator token and longest-match conflict.

Required examples include:

```text
:
:=
:<-
<
<=
<-
<<
<<=
.
..
..<
...
!
!=
&
&&
&=
|
||
|=
```

## Parser precedence tests

Test every adjacent precedence pair.

Examples:

```sec
a + b * c
a << b + c
a == b & c
a & b == c
a || b && c
a x b + c
value.Member()[index].Field
```

## Associativity tests

```sec
a - b - c
a / b / c
a << b << c
a x b x c
```

Verify comparisons do not chain.

## Evaluation-order tests

Use observable test helpers to verify:

```text
binary operands
call arguments
index base and index
struct fields
array elements
spread
compound assignment target
```

## Arithmetic tests

Test every integer width and signedness for:

```text
minimum
maximum
zero
one
overflow
underflow
division by zero
minimum / -1
negative remainder
compound assignment
```

## Float tests

Test:

```text
finite values
NaN
positive infinity
negative infinity
positive zero
negative zero
remainder special cases
all comparison operators
```

## Decimal tests

Test:

```text
exact remainder
negative operands
zero divisor
precision failure
range failure
compound remainder
```

## Shift tests

Test every integer width:

```text
count 0
count width - 1
count width
negative count
runtime invalid count
signed right shift
unsigned right shift
signed left overflow
unsigned truncating left shift
```

## Equality tests

Test:

```text
bool
integer
float NaN
char
rune
string
enum
array
struct with impl
struct with resource field
union
slice rejection
interface rejection
```

## Ordering tests

Test:

```text
numeric
char
rune
string prefix
string Unicode scalar ordering
NaN
enum rejection
array rejection
struct rejection
```

## Membership tests

Test:

```text
inclusive range
exclusive range
empty range
fixed array first match
fixed array no match
slice first match
slice no match
short-circuit element comparison
unsupported list
unsupported map
unsupported string
```

## Assignment tests

Test:

```text
copy initialization
move initialization
copy assignment
move assignment
temporary initialization
compound target evaluated once
contract failure
overflow failure
overlap rejection
self-move rejection
```

## Formatter tests

Test canonical spacing and:

```text
x identifier versus matrix operator
++ normalization
-- normalization
copy versus move preservation
parentheses
multiline expressions
```

---

# Required source test files

Create or update:

```text
operators_valid.sec
operators_invalid.sec
operators_precedence_valid.sec
operators_precedence_invalid.sec
operators_arithmetic_valid.sec
operators_arithmetic_invalid.sec
operators_comparison_valid.sec
operators_comparison_invalid.sec
operators_membership_valid.sec
operators_membership_invalid.sec
operators_assignment_valid.sec
operators_assignment_invalid.sec
operators_shaped_valid.sec
operators_shaped_invalid.sec
```

Every invalid test includes:

```sec
/* Expected error: ...
 * Reason: ...
 */
```

---

# Required synchronization

This rulebook must remain synchronized with:

```text
lexical_structure.md
grammar.md
parser_recovery.md
formatter.md
ownership.md
copy_move.md
memory_model.md
types.md
contracts.md
functions.md
collections.md
shaped-types.md
declarations/spread.md
enums.md
unions.md
struct.md
types/units.md
references.md
raw_pointers.md
allocation.md
destruction.txt
concurrency_memory_model.txt
semantic_ir.md
compiler_pipeline.md
diagnostics.txt
lsp.md
language-rulebook-status.md
rules_implementations.txt
```

Files that do not yet exist remain planned dependencies.

---

# Appendix A — Codex implementation plan

## A.1 Add the canonical rulebook

Add:

```text
rules/foundations/operators.md
```

Update:

```text
language-rulebook-status.md
rules_implementations.txt
```

Mark the document as written.

---

## A.2 Create one precedence definition

Move precedence metadata into one compiler package shared by parser tests,
formatter, LSP, and documentation generation where practical.

Do not maintain unrelated precedence tables.

The parser may retain efficient constants generated from the canonical table.

---

## A.3 Add contextual `x`

Parser and fixed-shape Sema support are implemented.

Identifier token `x` becomes an infix operator only when:

- parser is in infix position;
- the surrounding token structure is valid;
- Sema resolves compatible shaped operands.

Preserve ordinary identifiers named `x`.

Formatter and semantic-token context, Semantic IR and backend lowering remain.

---

## A.4 Enforce strict evaluation order

Audit:

```text
parser AST order
Sema effect order
Semantic IR
direct LLVM backend
MLIR backend
optimizations
```

Represent observable left-to-right order explicitly where needed.

Add exact-once compound-target tests.

---

## A.5 Preserve short-circuit lowering

Retain current CFG-based implementation for:

```text
&&
||
```

Extend Sema ownership, borrow, effect, and lifetime merging across the right
operand.

Add Semantic IR short-circuit nodes or explicit blocks.

---

## A.6 Implement checked integer arithmetic

Add checks for:

```text
+
-
*
unary -
signed <<
division minimum / -1
compound forms
```

Compile-time constants use arbitrary-precision evaluation and target type range
checks.

Runtime lowering uses target operations plus explicit failure edges.

Do not rely on LLVM undefined signed overflow.

---

## A.7 Division and remainder

Implement truncation-toward-zero integer division and remainder.

Add deterministic zero-divisor checks.

Add:

```text
float truncation-based remainder
decimal truncation-based remainder
```

Preserve special float cases.

---

## A.8 Fix float `!=`

Current ordered-not-equal lowering is incompatible with:

```text
NaN != NaN == true
```

Use unordered-not-equal or equivalent explicit logic.

Verify every float predicate against this rulebook.

---

## A.9 Implement shift validation

Before lowering:

- prove or check `0 <= count < width`;
- use `ashr` for signed right shift;
- use `lshr` for unsigned right shift;
- use ordinary fixed-width shift for unsigned left;
- check signed left-shift representability.

Do not mask counts silently.

---

## A.10 Ordered comparison validation

Update Sema so `<`, `<=`, `>`, and `>=` accept only:

```text
compatible numeric values
char with char
rune with rune
string with string
```

Reject enums, arrays, slices, structs, unions, references, and interfaces unless
a future rule adds support.

---

## A.11 String comparison

Implement exact string equality and Unicode-scalar lexicographic ordering.

Do not add:

```text
locale collation
normalization
case folding
```

Avoid allocation.

---

## A.12 Struct equality

Implement derived structural equality.

Requirements:

- all stored fields equality-comparable;
- normal `impl` does not disable derivation;
- no padding comparison;
- declaration-order comparison;
- short-circuit first mismatch;
- reject explicit identity/resource/non-comparable types.

Add temporary internal metadata until canonical attribute syntax is defined.

---

## A.13 Interface equality restriction

Reject direct equality on general interface values in Sec 0.1.

Do not compare erased descriptor bits or addresses.

---

## A.14 Array and slice membership

Sema implements and validates:

```text
value in fixedArray
value in slice
```

Lower as left-to-right short-circuit search.

Evaluate the left value and collection expression exactly once.

Do not allocate.

Do not use arbitrary `.Contains` method dispatch.

Semantic IR and backend lowering remain pending.

---

## A.15 Preserve range-only range values

Continue contextual parsing of:

```text
for ranges
membership ranges
slice ranges
```

Reject standalone range values in Sec 0.1.

Add a focused diagnostic.

---

## A.16 Complete runtime string concatenation

1. Accept the canonical direct operand matrix.
2. Reject non-text operands without hidden conversion.
3. Recognize interpolation as an explicit formatting context.
4. Resolve the active allocation context.
5. Select panic or `AllocationError` flow from source `try` context.
6. Enforce `@noPanic`.
7. Construct one maximal `StringConcatPlan`.
8. Preserve left-to-right exact-once evaluation.
9. Perform checked total-length calculation.
10. Allocate canonical result storage at most once.
11. Support transactional string `+=`.
12. Route `string.Concat` through the same plan.
13. Keep `S1020` retired and register replacement diagnostics.
14. Add LSP fixes and structural performance information.
15. Lower through Sec MLIR without nested binary allocation semantics.

---

## A.17 Question mark diagnostic

Keep `QUESTION` token.

Reject it in parser expression contexts with:

```text
`?` is reserved and has no meaning in Sec 0.1
```

Do not assign error propagation, optional access, or ternary semantics.

---

## A.18 Compound assignments

Audit every compound operator for:

```text
target evaluated once
source evaluated once
checked arithmetic
contracts
try
ownership
volatile access
atomic restrictions
failure commit order
```

Represent compound assignment semantically rather than by AST duplication.

---

## A.19 Semantic IR

Add resolved operations and metadata from this rulebook.

Ensure no generic operator token reaches MLIR without semantic classification.

---

## A.20 Formatter

Update `internal/formatter` when created.

Support:

```text
canonical spacing
contextual x
move operators
++ and -- normalization
parentheses from precedence
recoverable expressions
```

Do not duplicate precedence logic.

---

## A.21 LSP

Expose:

```text
operator hover
precedence
result type
failure behavior
short-circuit explanation
overflow explanation
unit result
matrix shape result
quick fixes
```

Use compiler-owned operator facts.

---

## A.22 Diagnostics

Register stable IDs.

Attach:

```text
operand ranges
resolved operand types
expected categories
failure cause
safe fixes
related declarations
```

---

## A.23 Backend audit

Audit direct LLVM and MLIR implementations against the same test matrix.

Do not allow backend feature divergence to become source-language divergence.

A backend may report unsupported lowering.

It may not silently produce different semantics.

---

## A.24 Recommended implementation order

```text
1. Add canonical operator metadata and tests.
2. Add contextual `x` Semantic IR, lowering and tooling context.
3. Enforce strict evaluation order and exact-once targets.
4. Complete checked integer arithmetic.
5. Complete division, remainder, and shifts.
6. Fix float comparisons.
7. Restrict ordered comparisons.
8. Add string comparison.
9. Add struct equality.
10. Add array and slice membership. (Frontend complete; Semantic IR and
    lowering remain.)
11. Complete runtime concat allocation/effect analysis and
    `StringConcatPlan`. (Direct operand frontend is complete.)
12. Add question-mark diagnostic.
13. Complete Semantic IR metadata.
14. Synchronize formatter and LSP.
15. Audit all backends.
```

---

# Design summary

Sec operators have one canonical precedence table.

All ordinary expression operands evaluate strictly left to right.

`&&` and `||` use lazy short-circuit evaluation.

Integer arithmetic is checked.

Integer division truncates toward zero.

`%` uses truncation-based remainder for integers, floats, and decimals.

Shifts use strict counts.

Signed right shift is arithmetic.

Unsigned right shift is logical.

Unsigned left shift discards high bits.

Signed left shift is checked.

`char`, `rune`, and `string` support exact deterministic ordering.

Strings order lexicographically by Unicode scalar sequence.

Structs derive structural equality when their stored fields are comparable and
the type has no identity or opaque-resource semantics.

A normal `impl` does not disable equality.

Ranges remain contextual in Sec 0.1.

`in` supports ranges, fixed arrays, and slices.

`for value in source` is iteration grammar, not membership.

String `+` accepts exactly built-in `string`, `char`, and `rune` operands and
always produces `string`; other values require explicit conversion, while
interpolation is an explicit formatting context.

Compile-time concatenation folds without allocation. Runtime concatenation uses
the active allocation context and selects deterministic allocation panic by
default or `AllocationError` flow through `try`. Maximal concatenation,
interpolation, `string.Concat`, and transactional `string +=` share one
`StringConcatPlan` before MLIR.

`?` remains reserved with no Sec 0.1 meaning.

Arbitrary user-defined operator declarations are not supported.

Sema resolves every operator completely.

Semantic IR preserves the resolved operation.

MLIR and backends implement Sec semantics without changing them.
