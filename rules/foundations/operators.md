# Operators

- **Status:** Normative
- **Created:** 2026-08-01
- **Last updated:** 2026-11-09 (§ 18.1(3) first-loop-iteration overflow warning); 2026-11-07 (stable chapter, subsection, and paragraph numbering)
- **Document revision:** 1
- **Sec language version:** 0.1
- **Canonical path:** `rules/foundations/operators.md`
- **Repository baseline reviewed:** `28780c01d63c3fc9eec2321ef67e3dd69962f043`
- **Implementation governance:** `governance/sema.yaml` (`frontend.operators-v2`)
- **Related rulebooks:** `rules/foundations/grammar.md`, `rules/foundations/lexical_structure.md`, `rules/types/types.md`, `rules/types/contracts.md`, `rules/types/default_values.md`, `rules/collections/collections.md`, `rules/memory/ownership.md`, `rules/memory/borrowing.md`, `rules/errors/runtime_checks.md`, `rules/errors/panic.md`, `rules/compiler/semantic_ir.md`, `rules/tooling/formatter.md`, `rules/tooling/lsp.md`

---

## § 1 Status

**§ 1(1)** This document is the canonical operator rulebook for Sec.

**§ 1(2)** It defines:

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

**§ 1(3)** This document must remain synchronized with the actual lexer, parser, AST,
Sema, formatter, Semantic IR, MLIR, and backend implementations.

**§ 1(4)** `default` is a named-type declaration clause, not an operator. Expressions in
an explicit default use the constant-expression subset of the operator rules in
this document. An invalid constant operation invalidates the default, and the
expression may not allocate or depend on runtime state. See
`default_values.md`.

---

## § 2 Current implementation status

**§ 2(1)** The Sec compiler already implements a substantial operator foundation.

**§ 2(2)** The current implementation must be preserved where it agrees with this
rulebook and corrected where it does not.

### § 2.1 Implemented

**§ 2.1(1)** The lexer currently recognizes:

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

**§ 2.1(2)** The lexer uses longest-match behavior for multi-character tokens.

**§ 2.1(3)** The parser currently implements a Pratt-style expression parser with precedence
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

**§ 2.1(4)** The parser currently supports:

- unary `+`;
- unary `-`;
- logical negation `!`;
- bitwise complement `~`;
- prefix dereference `*` of a safe reference (MD-063);
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

**§ 2.1(5)** The AST currently preserves:

- infix operator spelling;
- prefix operator spelling;
- assignment operator spelling;
- ownership mode for declarations and assignments.

**§ 2.1(6)** Sema currently implements:

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
- interpolation-hole formatting-contract validation with compiler-known and
  exact user-owned `ToString() Result[string, StringError]` selection through stable diagnostic
  `S1029`;
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

**§ 2.1(7)** The direct LLVM backend already implements lazy short-circuit control flow for:

```text
&&
||
```

**§ 2.1(8)** The MLIR backend already lowers several:

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

### § 2.2 Partially implemented

**§ 2.2(1)** The following are only partially implemented:

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
- runtime concatenation is type-checked, interpolation formatting contracts are
  selected, and mixed concat/interpolation expressions produce one immutable
  maximal frontend `StringConcatPlan`; allocation-context resolution,
  `try`-selected failure flow, `@noPanic` enforcement, Semantic IR consumption,
  and lowering are not implemented;
- `string +=` accepts the direct text operand matrix, but transactional
  fallible commit semantics and in-place proof optimization are not implemented;
- compound assignment does not yet implement every required check and
  evaluation-order guarantee;
- ordinary formatter support exists for many operators but does not yet cover
  every canonical normalization and move token;
- parser recovery for malformed operator expressions is incomplete;
- operator diagnostics are not all registered with stable IDs.

**§ 2.2(2)** The implemented structured operator checks emit:

```text
S1016  operator.non-orderable-operands
S1017  operator.invalid-shift-count
S1018  operator.signed-left-shift-overflow
S1019  operator.non-comparable-operands
S1021  operator.invalid-membership
S1022  operator.invalid-concat-operand
S1029  operator.invalid-interpolation-value
```

**§ 2.2(3)** `S1020 operator.string-runtime-concat` is retired and its numeric ID remains
reserved. It must not be emitted or reused.

### § 2.3 Not implemented

**§ 2.3(1)** The following are not yet implemented completely:

- one generated or shared canonical precedence definition;
- lowering and tooling context for matrix operator `x`;
- complete checked integer arithmetic;
- complete deterministic arithmetic-failure lowering;
- float `%`;
- decimal `%`;
- decimal result-scale selection, rounded division, provably-zero-divisor
  rejection and the `ScaleOverflow`/`PrecisionExhausted` classification
  (MD-066/MD-067, 2026-10-28);
- fixed-array and slice membership Semantic IR and lowering;
- materialization of accepted compile-time string concatenation as one folded
  constant before Semantic IR/lowering;
- active allocation-context resolution for runtime concatenation;
- rejection of runtime interpolation without `try`, and of an unhandled
  `Result[string, StringError]` concatenation operand; runtime `+` and `+=`
  select `try` or panic from source (MD-039);
- Semantic IR and lowering consumption of the canonical frontend
  `StringConcatPlan`;
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

## § 3 Purpose

**§ 3(1)** Operators provide concise syntax for common semantic operations.

**§ 3(2)** An operator does not gain meaning solely from its token.

**§ 3(3)** Its resolved meaning depends on:

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

**§ 3(4)** No backend phase may reinterpret an operator differently from Sema.

---

## § 4 Scope

**§ 4(1)** This rulebook defines ordinary source operators.

**§ 4(2)** Specialized rulebooks define additional details for:

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

concurrency_memory_model.md
    atomics and ordering

memory_model.md
    places, storage, and valid access
```

---

## § 5 Definitions

### § 5.1 Operator

**§ 5.1(1)** An operator is source syntax selecting a compiler-defined semantic operation.

**§ 5.1(2)** Examples:

```sec
left + right
!enabled
value[index]
object.Member
destination <- source
```

### § 5.2 Operand

**§ 5.2(1)** An operand is an expression or place consumed, copied, borrowed, inspected, or
mutated by an operator.

### § 5.3 Operator result

**§ 5.3(1)** An operator result may be:

```text
a value
a place
a reference
a boolean
a range context
a statement effect
no value
```

**§ 5.3(2)** Assignment operators do not produce expression values in Sec 0.1.

### § 5.4 Operator family

**§ 5.4(1)** Operators are grouped into:

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

## § 6 General rules

### § 6.1 Compiler-defined meaning

**§ 6.1(1)** Sec 0.1 does not support arbitrary user-defined operator declarations.

**§ 6.1(2)** A user cannot declare:

```sec
operator +(left: MyType, right: MyType) MyType
```

**§ 6.1(3)** Compiler-known and core-known types may support operators through language
rules.

**§ 6.1(4)** Examples include:

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

**§ 6.1(5)** This is not arbitrary operator overloading.

### § 6.2 Exact operator resolution

**§ 6.2(1)** Operator resolution is completed during Sema.

**§ 6.2(2)** The resolved operation records:

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

### § 6.3 No hidden fallback

**§ 6.3(1)** When an operator is invalid for its operands, the compiler must not silently:

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

### § 6.4 No target-dependent source meaning

**§ 6.4(1)** A target may reject unsupported operations.

**§ 6.4(2)** A target must not change:

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

## § 7 Canonical precedence

**§ 7(1)** The following table is ordered from highest precedence to lowest precedence.

| Level | Category | Operators and forms | Associativity |
|---:|---|---|---|
| 1 | Member | `.` | left chaining |
| 2 | Postfix | call `()`, index or slice `[]`, struct literal `{}`, spread `...` | left chaining |
| 3 | Prefix | unary `+`, unary `-`, `!`, `~`, dereference `*`, and parser-defined prefix forms | right |
| 4 | Multiplicative | `*`, `/`, `%`, contextual `x` | left |
| 5 | Additive | `+`, `-` | left |
| 6 | Shift | `<<`, `>>` | left |
| 7 | Ordered comparison and membership | `<`, `<=`, `>`, `>=`, `in`, `not in` | non-chainable |
| 8 | Equality and state test | `==`, `!=`, `is`, `is not` | non-chainable |
| 9 | Bitwise AND | `&` | left |
| 10 | Bitwise XOR | `^` | left |
| 11 | Bitwise OR | `|` | left |
| 12 | Logical AND | `&&` | left with short-circuit |
| 13 | Logical OR | `||` | left with short-circuit |

**§ 7(2)** Assignment is not part of expression precedence.

**§ 7(3)** Assignment is statement syntax.

---

## § 8 Precedence examples

```sec
a + b * c
```

**§ 8(1)** means:

```sec
a + (b * c)
```

```sec
a << b + c
```

**§ 8(2)** means:

```sec
a << (b + c)
```

```sec
a & b == c
```

**§ 8(3)** means:

```sec
a & (b == c)
```

**§ 8(4)** only if the resulting types are valid.

**§ 8(5)** In ordinary integer code, this will normally be a type error because equality
produces `bool`.

```sec
a == b & c
```

**§ 8(6)** means:

```sec
a == (b & c)
```

```sec
a || b && c
```

**§ 8(7)** means:

```sec
a || (b && c)
```

```sec
left x right + offset
```

**§ 8(8)** means:

```sec
(left x right) + offset
```

---

## § 9 Parentheses

**§ 9(1)** Parentheses override precedence.

```sec
(a + b) * c
```

**§ 9(2)** Parentheses may also document intent.

**§ 9(3)** The formatter may remove only provably redundant parentheses according to
`formatter.md`.

**§ 9(4)** Parentheses that preserve readability around mixed boolean or comparison
expressions may remain.

---

## § 10 Associativity

### § 10.1 Left-associative binary operators

**§ 10.1(1)** Unless explicitly stated otherwise, ordinary binary operators associate left.

```sec
a - b - c
```

**§ 10.1(2)** means:

```sec
(a - b) - c
```

```sec
a / b / c
```

**§ 10.1(3)** means:

```sec
(a / b) / c
```

```sec
a << b << c
```

**§ 10.1(4)** means:

```sec
(a << b) << c
```

### § 10.2 Prefix operators

**§ 10.2(1)** Prefix operators associate right.

```sec
!!value
```

**§ 10.2(2)** means:

```sec
!(!value)
```

### § 10.3 Non-chainable comparisons

**§ 10.3(1)** Comparisons are not mathematically chained.

**§ 10.3(2)** Invalid:

```sec
a < b < c
```

**§ 10.3(3)** Invalid:

```sec
a == b == c
```

**§ 10.3(4)** Write:

```sec
a < b && b < c
```

**§ 10.3(5)** or:

```sec
a == b && b == c
```

**§ 10.3(6)** The compiler must provide a focused diagnostic.

### § 10.4 State tests with `is`

**§ 10.4(1)** A non-binding `is` state test is an ordinary expression of type `bool` and may
appear in every expression context where a `bool` expression is valid:

```sec
let idle := state is Idle
let missing := option is None

return state is Running

if ready && state is Idle {
    Use()
}
```

**§ 10.4(2)** `is` belongs to the equality/state-test precedence level (level 8), so
`ready && state is Idle` groups as `ready && (state is Idle)`. Equality and
state-test operators at that level are non-chainable: `state is Idle == true`
is rejected, while the parenthesized `(state is Idle) == true` is valid but
redundant and receives a bad-practice diagnostic recommending `state is Idle`.

**§ 10.4(3)** `is not` is the canonical direct negation of a non-binding state test:

```sec
state is not Idle
state is not empty
option is not None
place is not available
```

**§ 10.4(4)** The exact designators valid for a type are defined by the rulebook that owns
that type or state operation. `!(state is Idle)` is valid but receives a
bad-practice diagnostic recommending `state is not Idle`. Ordinary prefix
precedence still applies: `!state is Idle` means `(!state) is Idle`.
Explicit Language Corrections may rewrite both bad-practice forms when the
rewrite is unambiguous and semantics-preserving.

**§ 10.4(5)** The foreign null tests `pointer is null` and `pointer is not null` are state
tests at this level owned by `rules/platform/ffi.md` § 11: they are valid only
inside `unsafe` with a raw-pointer operand, `is not null` is their canonical
negated spelling, and `== null`/`!= null` are invalid (MD-024).

**§ 10.4(6)** The positive Option binding `if option is Some(value)` remains a narrow Sec 0.1
condition facility (`rules/control-flow/flowcontrol_if.md` § 12). It is not a
general stored or returned bool expression, it does not generalize to other
union or Result variants, and it introduces no binding scope through `&&` or
`||`. `option is not Some(value)` and `!(option is Some(value))` are invalid
because their true path has no `Some` payload to bind; corrections never delete
the binding (MD-006; `rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md` § 7).

---

## § 11 Evaluation order

### § 11.1 General rule

**§ 11.1(1)** Sec evaluates expression components strictly from left to right.

**§ 11.1(2)** This applies to:

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

**§ 11.1(3)** The compiler may reorder physical instructions only when observable behavior is
unchanged.

**§ 11.1(4)** Observable behavior includes:

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

### § 11.2 Binary operator

```sec
First() + Second()
```

**§ 11.2(1)** evaluates:

1. `First()`;
2. `Second()`;
3. addition.

### § 11.3 Function arguments

```sec
Call(First(), Second(), Third())
```

**§ 11.3(1)** evaluates arguments in source order.

### § 11.4 Index

```sec
values[NextIndex()]
```

**§ 11.4(1)** evaluates:

1. `values`;
2. `NextIndex()`;
3. bounds validation;
4. access.

### § 11.5 Struct literal

```sec
let value := Record {
    First: CreateFirst(),
    Second: CreateSecond(),
}
```

**§ 11.5(1)** evaluates fields in source order.

### § 11.6 Spread

**§ 11.6(1)** A spread source is evaluated exactly once.

**§ 11.6(2)** Expanded elements are processed in natural source order.

---

## § 12 Exact-once place evaluation

**§ 12(1)** An assignment target or compound-assignment target is evaluated exactly once.

**§ 12(2)** Example:

```sec
values[NextIndex()] += NextValue()
```

**§ 12(3)** Conceptual order:

1. evaluate `values`;
2. evaluate `NextIndex()` exactly once;
3. resolve and validate the target place;
4. read the old target value;
5. evaluate `NextValue()`;
6. perform addition;
7. validate overflow, contracts, and other fallible conditions;
8. write the new value.

**§ 12(4)** The compiler must not lower this as:

```sec
values[NextIndex()] = values[NextIndex()] + NextValue()
```

**§ 12(5)** because that would evaluate the target more than once.

---

## § 13 Short-circuit evaluation

### § 13.1 Logical AND

```sec
left && right
```

**§ 13.1(1)** evaluates `left` first.

**§ 13.1(2)** When `left` is `false`:

- `right` is not evaluated;
- the result is `false`.

**§ 13.1(3)** When `left` is `true`:

- `right` is evaluated;
- the result is the value of `right`.

### § 13.2 Logical OR

```sec
left || right
```

**§ 13.2(1)** evaluates `left` first.

**§ 13.2(2)** When `left` is `true`:

- `right` is not evaluated;
- the result is `true`.

**§ 13.2(3)** When `left` is `false`:

- `right` is evaluated;
- the result is the value of `right`.

### § 13.3 Path-sensitive effects

**§ 13.3(1)** A skipped right operand does not:

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

**§ 13.3(2)** Ownership, borrowing, effects, and diagnostics must follow control flow.

---

## § 14 Primary and postfix forms

### § 14.1 Member access

```sec
value.Member
```

**§ 14.1(1)** Member access has the highest precedence.

**§ 14.1(2)** The left expression is resolved before the member.

**§ 14.1(3)** Member access may resolve:

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

**§ 14.1(4)** A member access is a place only when the resolved member and receiver allow it.

### § 14.2 Call

```sec
Function(argument1, argument2)
```

**§ 14.2(1)** Call syntax may represent:

```text
function call
method call
constructor-like compiler operation
type conversion
generic instantiation followed by call
```

**§ 14.2(2)** Sema determines the meaning.

**§ 14.2(3)** The surface syntax alone does not determine whether a form is a function call or
conversion.

### § 14.3 Conversion

```sec
Percent(value)
```

**§ 14.3(1)** may resolve as a type conversion when `Percent` is a type.

**§ 14.3(2)** Conversion rules come from `types.md`, named-type rules, units, and contracts.

**§ 14.3(3)** A conversion is not an ordinary user-overloadable call.

### § 14.4 Index

```sec
values[index]
```

**§ 14.4(1)** Indexing is postfix syntax.

**§ 14.4(2)** It may resolve for:

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

**§ 14.4(3)** Exact indexing semantics belong to the type's rulebook.

### § 14.5 Slice

```sec
values[start..<end]
```

**§ 14.5(1)** Slicing is contextual postfix syntax.

**§ 14.5(2)** Slice bounds evaluate left to right.

### § 14.6 Struct literal

```sec
Point {
    X: 1,
    Y: 2,
}
```

**§ 14.6(1)** The parser distinguishes a struct literal from an ordinary block by grammar
context.

### § 14.7 Spread

```sec
values...
```

**§ 14.7(1)** Spread is postfix syntax.

**§ 14.7(2)** Spread is valid only in approved contexts.

**§ 14.7(3)** Sec 0.1 contexts include:

```text
function arguments
array literals
struct literals
```

**§ 14.7(4)** Spread is not a general value-producing operator.

---

## § 15 Prefix operators

### § 15.1 Unary plus

```sec
+value
```

**§ 15.1(1)** Unary plus is valid for numeric values.

**§ 15.1(2)** It preserves the numeric value and resolved type.

**§ 15.1(3)** It does not convert unsigned to signed or vice versa.

**§ 15.1(4)** It may be removed by the formatter only where doing so is canonical and does
not affect literal interpretation.

### § 15.2 Unary minus

```sec
-value
```

**§ 15.2(1)** Unary minus is valid for:

```text
signed integers
floating-point values
decimal values
compatible named numeric types
compatible unit-bearing numeric values
```

**§ 15.2(2)** Unary minus is invalid for ordinary unsigned runtime values.

**§ 15.2(3)** A negative untyped literal may be shaped directly into a signed compatible
context.

**§ 15.2(4)** Signed integer negation is checked.

**§ 15.2(5)** Negating the minimum representable signed integer causes:

```text
compile-time error
    when known during compilation

deterministic arithmetic failure
    when detected at runtime
```

**§ 15.2(6)** Floating negation follows floating-point sign semantics.

**§ 15.2(7)** Decimal negation follows checked decimal representability.

### § 15.3 Logical negation

```sec
!value
```

**§ 15.3(1)** Operand type must be `bool`.

**§ 15.3(2)** Result type is `bool`.

**§ 15.3(3)** No numeric truthiness exists.

**§ 15.3(4)** Invalid:

```sec
!1
```

### § 15.4 Bitwise complement

```sec
~value
```

**§ 15.4(1)** Operand must be an integer type.

**§ 15.4(2)** Result type is the operand type.

**§ 15.4(3)** The operation flips every value bit in the fixed-width representation.

**§ 15.4(4)** For a signed integer, interpretation of the result uses the same signed type.

### § 15.5 Dereference

```sec
*reference
```

**§ 15.5(1)** The operand must be a safe `ref T` or `ref mut T`. The expression names the
referent Place of type `T` (`rules/memory/references.md` § 3.4; MD-063).

**§ 15.5(2)** A value use reads the referent by value and requires `T` to be copyable. The
operator never moves out of a referent, and `:<- *r` / `<- *r` are invalid.

**§ 15.5(3)** `*r` is a valid assignment and compound-assignment target when `r` is
`ref mut T`; a `ref T` grants no write authority.

**§ 15.5(4)** `*` in prefix position is dereference; in infix position it is
multiplication. Postfix forms bind tighter than prefix forms, so:

```text
*r.Field    means *(r.Field)
*r * 2      means (*r) * 2
(*r).Field  projects Field from the explicit referent
```

**§ 15.5(5)** `Option[ref T]`, `RawPtr[T]`, slices and plain values are not valid operands.

**§ 15.5(6)** No operator dereferences a reference implicitly. `r + 1`, `r += 1` and
`r = value` act on the holder `r` and are rejected when they need the
referent; the diagnostic suggests `*r`.

**§ 15.5(7)** `r == s` compares reference identity; `*r == *s` compares referent values.

### Line-start dereference statements

**§ 15.5(8)** Statements are not separated by newlines, and the canonical layout of a
multiline product puts `*` at the start of a continuation line (formatter
§ 9(13)). The parser decides between the two readings of a `*` that begins a
later line syntactically, never from types:

1. The `*` is a dereference that begins a new statement when it introduces a
   syntactically valid assignment target and that target is directly
   followed by an assignment operator, or by the statement alias `++` / `--`.
   A target is one or more `*` followed by an identifier, `self` or a
   parenthesized expression, then any member (`.Name`), index (`[...]`) and
   call (`(...)`) steps.
2. The search for the assignment operator may cross lines inside the
   target's own brackets, but never continues past the end of the target
   into another standalone statement.
3. In every other case the `*` keeps its existing meaning as a
   multiplication on a continuation line.

```sec
let total :=
      base
    * factor          // multiplication continuation
*r = total            // new dereference assignment statement
*grid[
    index
] = 5                 // target brackets span lines
```

**§ 15.5(9)** Rule 1 only reinterprets source in which the multiplication reading would be
an assignment chained onto the previous statement, which is always invalid,
so it changes the meaning of no valid program.

---

## § 16 Arithmetic operators

**§ 16(1)** Arithmetic operators include:

```text
+
-
*
/
%
```

**§ 16(2)** They may be available for:

```text
integers
floats
decimals
named numeric types
unit-bearing numeric types
compiler-known shaped values where defined
```

**§ 16(3)** No implicit conversion between unrelated nominal types is introduced merely to
make an operator valid.

---

## § 17 Numeric compatibility

**§ 17(1)** Operands must be type-compatible according to numeric conversion rules.

**§ 17(2)** The compiler may shape untyped literals into a compatible operand type.

**§ 17(3)** Examples:

```sec
let value: int32 := 10
let result := value + 2
```

**§ 17(4)** The literal `2` may shape to `int32`.

**§ 17(5)** No implicit conversion occurs between unrelated named types.

```sec
type Meters int
type Seconds int

let distance: Meters := Meters(10)
let time: Seconds := Seconds(2)
```

**§ 17(6)** This is invalid unless a unit or explicit operator rule defines it:

```sec
distance + time
```

---

## § 18 Checked integer arithmetic

**§ 18(1)** Integer arithmetic is checked by default.

**§ 18(2)** This includes:

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

### § 18.1 Compile-time overflow

**§ 18.1(1)** When the local constant analysis indicates integer overflow, the compiler emits
the `S1023` warning. The warning does not block compilation because a dominating
flow proof may establish that the operation is safe even when that proof is not
available at the expression itself.

**§ 18.1(2)** The accepted operation retains ordinary checked runtime semantics. Suppressing
or downgrading the warning must never turn overflow into wrapping or undefined
behavior.

**§ 18.1(3)** Inside a loop body, a binding the body assigns has no known constant
value, because later iterations see what earlier ones left. The local constant
analysis may still use the value from before the loop for the first iteration
when the loop condition can hold with that value; an overflow certain there
reports `S1023` naming the first loop iteration. A loop whose condition is false
on entry produces no such warning:

```sec
let mut n: uint := 3
while ready {
    n -= 4                     // S1023: may overflow uint on the first loop iteration
}

let mut m: uint := 0
while m > 0 {
    m -= 1                     // no warning: the body never runs with m == 0
}
```

### § 18.2 Runtime overflow

**§ 18.2(1)** When overflow depends on runtime values, the generated program performs a
deterministic arithmetic failure.

**§ 18.2(2)** Overflow must not:

```text
wrap silently
be undefined behavior
depend on optimization mode
depend on build profile
depend on target instruction behavior
```

### § 18.3 Explicit alternatives

**§ 18.3(1)** Core may provide explicit operations such as:

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

**§ 18.3(2)** Exact API names belong to core numeric rules.

---

## § 19 Integer addition

```sec
left + right
```

**§ 19(1)** Produces the mathematical sum when representable.

**§ 19(2)** Otherwise checked overflow behavior applies.

**§ 19(3)** Unsigned overflow and underflow are checked.

---

## § 20 Integer subtraction

```sec
left - right
```

**§ 20(1)** Produces the mathematical difference when representable.

**§ 20(2)** Unsigned underflow is an arithmetic failure.

---

## § 21 Integer multiplication

```sec
left * right
```

**§ 21(1)** Produces the mathematical product when representable.

**§ 21(2)** Otherwise checked overflow behavior applies.

---

## § 22 Integer division

**§ 22(1)** Integer division truncates toward zero.

**§ 22(2)** Examples:

```text
7 / 3 == 2
-7 / 3 == -2
7 / -3 == -2
-7 / -3 == 2
```

**§ 22(3)** Division by zero causes:

```text
compile-time error
    when divisor is known to be zero

deterministic arithmetic failure
    otherwise
```

**§ 22(4)** For a two's-complement signed type:

```text
minimumValue / -1
```

**§ 22(5)** is checked overflow.

---

## § 23 Integer remainder

**§ 23(1)** Integer `%` uses the quotient produced by truncation toward zero.

**§ 23(2)** Definition:

```text
quotient = trunc(left / right)
remainder = left - quotient * right
```

**§ 23(3)** Invariant:

```text
left == quotient * right + remainder
```

**§ 23(4)** Examples:

```text
7 % 3 == 1
-7 % 3 == -1
7 % -3 == 1
-7 % -3 == -1
```

**§ 23(5)** The remainder has the sign of the left operand or is zero.

**§ 23(6)** A zero divisor follows integer division-by-zero rules.

---

## § 24 Floating arithmetic

**§ 24(1)** Floating arithmetic follows the selected floating type's IEEE-compatible
semantics unless a target or type rule explicitly rejects an unsupported
operation.

**§ 24(2)** Floating arithmetic does not use integer checked-overflow semantics.

**§ 24(3)** Possible floating results include:

```text
finite value
positive infinity
negative infinity
NaN
positive zero
negative zero
```

**§ 24(4)** The compiler must preserve the language's comparison and remainder rules.

**§ 24(5)** A build profile must not silently switch to unsafe fast-math semantics that
change observable Sec results.

**§ 24(6)** Any future fast-math mode must be explicit and separately specified.

---

## § 25 Floating division

**§ 25(1)** Floating division follows floating-point semantics.

**§ 25(2)** Examples conceptually include:

```text
finite / zero
    signed infinity where defined

zero / zero
    NaN

infinity / infinity
    NaN
```

**§ 25(3)** Exact results follow the selected floating format.

---

## § 26 Floating remainder

**§ 26(1)** Floating `%` uses truncation-based remainder.

**§ 26(2)** Definition for finite operands:

```text
quotient = trunc(left / right)
remainder = left - quotient * right
```

**§ 26(3)** This is not IEEE nearest-integer `remainder`.

**§ 26(4)** Examples:

```text
7.5 % 2.0 == 1.5
-7.5 % 2.0 == -1.5
7.5 % -2.0 == 1.5
```

**§ 26(5)** Special cases:

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

**§ 26(6)** The result uses the common resolved floating type.

---

## § 27 Decimal arithmetic

**§ 27(1)** Decimal arithmetic follows exact decimal semantics and the selected decimal
type's precision and scale rules (MD-066/MD-067, 2026-10-28,
`rules/corrections/applied/md066-md067-md068-decimal-correction-20261028.md`).

**§ 27(2)** Operations are checked.

**§ 27(3)** `+`, `-`, `*`, `%` and unary minus produce the exact mathematical result or
fail. Division is the only decimal operator that may approximate its result
automatically, and only as defined in "Decimal division" below. No other
operator, implicit conversion or unit conversion gains a rounding permission
from that rule.

**§ 27(4)** Each result is computed exactly before its representation is selected. No
host binary float, fixed host 64-bit intermediate or overflow-driven
arithmetic defines the result. A decimal result never widens from `decimal`
to `decimal128` and never converts to binary float.

### § 27.1 Decimal result scale

**§ 27.1(1)** The value of a decimal is `coefficient × 10^(-scale)`, with a signed
`i64` (`decimal`) or `i128` (`decimal128`) coefficient and a signed `int32`
scale (`rules/memory/layout.md` § 19; `core-library.md` § 10.1).

**§ 27.1(2)** Each operation has a preferred scale `P`, computed without wrapping:

| Operation | Preferred scale `P` |
|---|---|
| `a + b`, `a - b` | `max(a.Scale, b.Scale)` |
| `a * b` | `a.Scale + b.Scale` |
| `a / b` | `max(a.Scale, b.Scale)` |
| `a % b` | `max(a.Scale, b.Scale)` |

**§ 27.1(3)** For a nonzero exactly representable result `V`, the exact scales at which an
integer coefficient in the actual signed coefficient domain represents `V`
form a contiguous interval `[S_min(V), S_max(V)]` within `int32`. The
selected scale is the member closest to `P`:

```text
resultScale = clamp(P, S_min(V), S_max(V))
```

**§ 27.1(4)** `clamp` here is mathematical, not `decimal.Clamp()`. When `P` lies inside the
interval it is kept; trailing coefficient zeros are not stripped
gratuitously. An operation is not rejected merely because its preferred
coefficient overflows when another exact scale exists. The coefficient domain
is the real asymmetric signed one: `-2^63` and `-2^127` are valid
coefficients.

**§ 27.1(5)** An exact zero result has coefficient zero and keeps `P`, clamped to
`int32.Min` or `int32.Max` when `P` lies outside `int32`; zero never fails
because an intermediate preferred scale, such as a product's scale sum, is out
of range.

**§ 27.1(6)** When no exact representable coefficient/scale pair exists, `+`, `-`, `*`,
`%` and unary minus fail as described in "Decimal arithmetic failures".

**§ 27.1(7)** Equality and ordering compare mathematical values: `1.2m == 1.20m`. Scale is
not part of equality or ordering, and comparison never goes through binary
floating point.

**§ 27.1(8)** Compound assignments use the same numerical policy as the corresponding
binary operator, with exactly-once operand evaluation and the owning
assignment rules for transactional updates.

### § 27.2 Decimal division

**§ 27.2(1)** The quotient is computed exactly. When it has an exact representation in the
selected decimal type, the result-scale rule above applies; an exact finite
quotient is never rounded or padded with extra digits.

**§ 27.2(2)** When no exact representation exists, whether the quotient is nonterminating
or merely needs more digits than the coefficient holds, division rounds:

- it selects the finest signed `int32` scale whose correctly rounded
  coefficient fits the signed coefficient domain and keeps the whole-number
  part;
- it rounds once from the exact quotient with `RoundingMode.NearestEven`, an
  exact midpoint selecting the even coefficient;
- if the rounding carry makes the coefficient unrepresentable, the next
  coarser admissible scale is tried; there is no wrapping, saturation, host
  float intermediate or double rounding.

**§ 27.2(3)** The whole-number magnitude takes precedence over fractional precision: the
scale is never coarsened so far that significant whole-number digits are
lost. An ordinary carry from rounding the fraction, such as a value just below
`1` rounding to `1`, is allowed. When no admissible representation keeps the
whole-number digits, division fails with the most specific `ArithmeticError`.
A quotient with zero whole-number part may round to zero when no nonzero
candidate is representable.

**§ 27.2(4)** A nonzero quotient that rounds to zero keeps the selected rounding scale; the
exact-zero preferred-scale rule applies only to an exactly zero quotient.

**§ 27.2(5)** Representative `decimal` results:

| Operation | Value | Result scale | Reason |
|---|---|---:|---|
| `1m / 3m` | `0.3333333333333333333` | 19 | Nonterminating; maximal precision. |
| `2m / 3m` | `0.6666666666666666667` | 19 | Nonterminating; nearest-even rounding. |
| `1000000000m / 3m` | `333333333.3333333333` | 10 | Nine whole digits; the remaining coefficient capacity is fractional. |
| `1m / 8m` | `0.125` | 3 | Exactly representable; no padding. |

**§ 27.2(6)** `decimal128` uses the same algorithm with its 128-bit coefficient; the
default precision never depends on the target.

**§ 27.2(7)** A rounded quotient is an ordinary successful value. It is not `PrecisionLoss`
and needs no `try` because it is inexact; `try` concerns only real failure
paths such as a runtime zero divisor or an unrepresentable whole-number part.

### § 27.3 Decimal division and remainder by zero

**§ 27.3(1)** When the compiler proves that the divisor of decimal `/` or `%` is zero, the
expression is a compile-time error, with or without `try`. Compilation stops
before successful code generation and reports the source location with a
repair hint. This rule concerns a provably zero decimal divisor only; it does
not make every provably failing runtime check a compile-time error.

**§ 27.3(2)** Otherwise the divisor is checked at run time. A zero divisor in the ordinary
operation reaches the non-returning panic endpoint with reason
`DivisionByZero`. Under `try a / b` or `try a % b` the failure is
`ArithmeticError.DivisionByZero` through ordinary `Result` propagation, with
no panic at that point.

**§ 27.3(3)** A proof that the divisor is nonzero may remove the check. The compiler never
assumes nonzero merely because ordinary division is written or because a
constant could not be evaluated in an optional compile-time context.

### § 27.4 Decimal arithmetic failures

**§ 27.4(1)** Decimal operators report `ArithmeticError`, choosing the most specific
applicable variant (`rules/errors/errorhandling.md` § 3.4):

| Semantic failure | Error |
|---|---|
| Runtime division or remainder by zero under `try` | `ArithmeticError.DivisionByZero` |
| A finite exact result needs a scale outside `int32` while the coefficient precision suffices | `ArithmeticError.ScaleOverflow` |
| An exact result needs more significant coefficient digits than the signed coefficient holds at any allowable exact scale | `ArithmeticError.PrecisionExhausted` |
| Any other unrepresentable outcome with no narrower applicable variant | `ArithmeticError.Overflow` |
| An inexact quotient successfully rounded by "Decimal division" | No failure |

**§ 27.4(2)** For an exactly representable finite decimal value, classification uses the
minimal exact coefficient after removing all factors of ten (for
classification only): if it does not fit the signed coefficient,
`PrecisionExhausted`; otherwise, if no `int32` scale represents the value,
`ScaleOverflow`; any remaining failure is `Overflow`. A rounded division that
cannot keep its whole-number part classifies the failed constraint the same
way. A nonterminating quotient is never `PrecisionExhausted` or
`PrecisionLoss` merely for being nonterminating. A runtime zero divisor is
detected before any quotient or remainder is attempted.

---

## § 28 Decimal remainder

**§ 28(1)** Decimal `%` uses truncation-based remainder.

**§ 28(2)** Definition:

```text
quotient = trunc(left / right)
remainder = left - quotient * right
```

**§ 28(3)** `quotient` is the exact mathematical integer truncated toward zero. It is
never obtained from the rounded public quotient of decimal division, and
`remainder` is computed exactly. Its scale follows "Decimal result scale".

**§ 28(4)** A decimal remainder is exact when representable; an unrepresentable remainder
fails as in "Decimal arithmetic failures", and a zero divisor follows
"Decimal division and remainder by zero".

**§ 28(5)** The result type follows the resolved decimal compatibility rules.

---

## § 29 String concatenation

**§ 29(1)** Runtime string concatenation with `+` is supported in Sec 0.1.

### § 29.1 Direct operand matrix

**§ 29.1(1)** The only direct operand types are the built-in, non-nominal types `string`,
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

**§ 29.1(2)** A `char` or `rune` already represents one textual element and does not require
an explicit conversion.

### § 29.2 No hidden general conversion

**§ 29.2(1)** The operator does not convert arbitrary operands to string. Numbers, `bool`,
enums, structs, unions, user-defined nominal types, arrays, slices,
collections, interfaces, and all other non-text values require explicit
conversion:

```sec
let valid := try "Count: " + count.ToString()
let invalid := "Count: " + count
```

### § 29.3 Fallible text operands

**§ 29.3(1)** Decision 2026-10-03: besides `string`, `char`, and `rune`, string `+` accepts an
operand of exactly `Result[string, StringError]`, such as a `ToString()` call.
The whole concatenation then is one runtime materialization whose single
`StringError` failure channel covers both the operand failures and the
allocation of the result, so one `try` around the complete expression handles
it:

```sec
let constant := "A" + "B"                        // compile-time text, no try
let text := try "A" + value.ToString() + "B"     // one try for the whole plan
let both := try left.ToString() + ":" + right.ToString()
```

**§ 29.3(2)** An operand of this type is never unwrapped implicitly: without a protecting `try`
the expression is rejected (S1097), not turned into a panic. A separately
protected operand handles only that operand, and the surrounding `+` stays in
ordinary panic mode:

```sec
let text := "Value: " + (try value.ToString())
```

**§ 29.3(3)** Each fallible operand evaluates exactly once, left to right, as one segment of
the maximal concatenation plan, and the first failure ends the plan. The operand
must be exactly `Result[string, StringError]`; any other `Result` or non-text
value remains invalid with S1022. Runtime concatenation and interpolation fail
with `StringError` (`rules/library/core-library.md` § 15.1); an allocation
failure is `StringError.Allocation`.

**§ 29.3(4)** Compound `string +=` admits the same operand under an outer `try`, where one
`StringError` channel covers the operand and the append (MD-039):

```sec
try text += value.ToString()     // one try for operand and append
text += try value.ToString()     // the operand's try; the append may panic
text += value.ToString()         // invalid: the Result is not handled (S1022)
```

**§ 29.3(5)** The compiler must not silently insert `ToString()`.

### § 29.4 Interpolation and formatting

**§ 29.4(1)** Interpolation is an explicit formatting context and is semantically distinct
from direct concatenation:

```sec
let text := $"Count: {count}"
```

**§ 29.4(2)** An interpolation hole is valid when its value has a canonical formatting
contract. Initially, a user-defined type normally supplies that contract through
`ToString()`. This does not make direct `"value: " + value` valid; direct
concatenation still requires the handled result of `value.ToString()`.

**§ 29.4(3)** Mixed interpolation and concatenation are valid. Interpolation formatting,
`value.ToString()`, and a future `value.ToString(format)` must share one coherent
formatting model. This rule does not lock the exact format syntax, format type,
or locale model.

### § 29.5 Compile-time concatenation

**§ 29.5(1)** When the complete concatenation is compile-time evaluable, it folds to one
string constant with:

```text
no runtime allocation
no allocation context requirement
no try required
no allocation-failure panic
no StringBuilder recommendation
```

**§ 29.5(2)** `try` on a fully folded concatenation is legal and has an unreachable failure
path (MD-039). This does not make unrelated infallible expressions valid `try`
operands.

### § 29.6 Runtime allocation and failure policy

**§ 29.6(1)** A runtime concatenation or interpolation expression is semantically one
materialization that produces one resulting `string`; the language requires no
observable intermediate `string` value for an individual concatenation step.

**§ 29.6(2)** Decided 2026-10-31 (MD-039; `rules/corrections/MD-004-MD-039-string-concatenation-try-or-panic-correction-20261031.md`).
Runtime `+` and `+=` have two failure modes selected by source syntax, never by
function return type, build mode, optimization, target or expected type:

```sec
let ordinary := "Hello " + name        // Materialization failure panics.
let recoverable := try "Hello " + name // Materialization failure is StringError.
```

**§ 29.6(3)** Under `try`, a storage failure is `StringError.Allocation(AllocationError)`,
handled or propagated by the ordinary rules. A failed materialization size check
follows the existing text classification and never wraps or truncates.

**§ 29.6(4)** Without `try`, storage materialization failure invokes a compiler-visible,
non-returning panic with the registered reason `StringMaterializationFailure`. It
returns no `Err`, does not propagate through the caller's `Result`, is not caught
by a `try` elsewhere, and reaches the allocation-free minimum panic path.

**§ 29.6(5)** The result value, exact-once left-to-right evaluation, ownership, lifetime,
allocation context and encoding do not depend on the selected mode.

**§ 29.6(6)** Runtime interpolation is unchanged by this decision: it is a fallible
materialization that requires `try`, and a concatenation containing an
interpolation requires it for that materialization.

**§ 29.6(7)** Runtime concatenation uses the canonical active allocation context defined by
`rules/memory/allocation.md`. Sec 0.1 defines no string-specific allocator and
no string-specific allocation domain. The frontend resolves the applicable
context before Semantic IR, and lowering consumes it without choosing an
allocator. When the result escapes a local allocation lifetime, the context's
lifetime must suffice for the result; a backend never repairs an invalid
lifetime by reallocating into another domain. Missing usable allocation context
is a compile-time error. On a target that prohibits dynamic allocation, a
runtime materialization is valid only when the compiler proves the dynamic
allocation is eliminated while preserving Sec semantics (MD-004; `rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md`
§§ 5.10–5.19).

| Form | May allocate | Failure without `try` | Failure under `try` |
|---|---:|---:|---:|
| Fully compile-time-folded concatenation | No | None | None |
| Runtime `+` or `+=` | Yes | Panic | `StringError` |
| Runtime `+` or `+=` with a `Result[string, StringError]` operand | Yes | Compile-time error | `StringError` |
| Runtime interpolation | Yes | Compile-time error | `StringError` |
| Runtime materialization without allocation context | Compile-time error in both modes | | |

**§ 29.6(8)** Optimization and backends may remove temporaries or allocations only when the
specified success, failure, ownership, and destruction behavior is preserved
(`rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md` § 5).

### § 29.7 `@noPanic` and `try` scope

**§ 29.7(1)** Under `@noPanic`, including restrictions inherited from interrupt contexts, an
unprotected runtime `+` or `+=` violates the guarantee unless a static proof shows
it needs no failing allocation. A protecting `try` satisfies it when `StringError`
is legally handled or propagated and no other panic effect is reachable. Under
`@noAlloc` both modes are rejected when an allocation effect is reachable: `try`
is not permission to allocate.

**§ 29.7(2)** `try` covers the concatenation's explicit fallible flow and nested operations
whose error flow is explicitly part of the expression. It does not catch an
arbitrary panic from operand evaluation or a user-defined `ToString()` call.
Those operations retain their declared effects and execute exactly once.

### § 29.8 Maximal concatenation plan

**§ 29.8(1)** A maximal concatenation and interpolation chain becomes one semantic
`StringConcatPlan`, not nested binary allocations. Conceptual segments include:

```text
ConstantStringSegment
StringSegment
CharSegment
RuneSegment
BuiltinFormattedSegment
MaterializedStringSegment
FallibleTextSegment
InterpolationSegment
```

**§ 29.8(2)** Compatible parenthesized text expressions may remain one plan. A separate
binding creates an observable semantic result; fusion across that boundary is
only an optimization after ownership, lifetime, and effect proof.

**§ 29.8(3)** Every segment evaluates left to right and exactly once. The plan may then
measure segment lengths, calculate the total with checked arithmetic, allocate
or otherwise prepare final storage once, and write segments in source order.
Static impossibility is a compile-time error; dynamic length failure follows the
plan's selected failure mode and never wraps silently.

**§ 29.8(4)** The canonical result is allocated at most once. User-defined `ToString()` calls
may allocate separate temporary strings; compiler-known formatting may be fused
without a temporary.

### § 29.9 Compound concatenation assignment

**§ 29.9(1)** `string +=` uses the same direct operand matrix and source-selected failure mode
as `+` (MD-039):

```sec
s += part        // Materialization failure panics.
try s += part    // Materialization failure is StringError.
```

**§ 29.9(2)** It is transactional: evaluate the destination place once, read the old value,
evaluate the appended operand once, prepare the new value, then commit, producing
no value. On failure while evaluating or materializing the appended value the
append commits nothing: under `try` the failure is recoverable, and in ordinary
mode materialization failure panics before commit. Independent side effects the
program caused while evaluating the operand are not undone.

**§ 29.9(3)** An outer `try` handles a `Result[string, StringError]` operand and the append as
one `StringError` channel; see "Fallible text operands".

**§ 29.9(4)** In-place append is permitted only after proof of unique ownership, sufficient
capacity, no conflicting alias, stable storage, and compatible allocation
context, and must preserve the selected failure mode and the transactional
commit. A `string +=` without `try` is a distinct ordinary panicking form of a
compiler-known string operation, not an unhandled fallible assignment; no general
non-`try` fallible assignment is introduced (`rules/errors/errorhandling.md`
§ 23).

### § 29.10 `string.Concat`

**§ 29.10(1)** `string.Concat` is an explicit core API that shares segment order, accepted
text operand categories, allocation context, try-or-panic selection, checked
length calculation, and canonical concat Semantic IR with `+`. Its exact
overload set belongs to the string/core rulebook; unrestricted variadics are not
defined here.

### § 29.11 `StringBuilder`

**§ 29.11(1)** `StringBuilder` is preferred for repeated incremental construction such as
loops, serialization, code generation, and unknown segment counts. It is not
required for one finite maximal concatenation expression.

**§ 29.11(2)** Repeated reconstruction may produce configurable information based on control
flow and mutation structure. The compiler must not inspect string content to
infer intent. Do not report one maximal expression, compile-time concatenation,
one isolated `+=`, a chain already planned as one allocation, proven in-place
reuse, or construction below the configured threshold.

---

## § 30 Unit-bearing arithmetic

**§ 30(1)** Unit-bearing numeric values use unit algebra.

**§ 30(2)** Examples:

```sec
distance + otherDistance
distance - otherDistance
speed * time
distance / time
```

**§ 30(3)** General rules:

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

**§ 30(4)** The complete unit algebra belongs to `rules/types/units.md`.

---

## § 31 Shaped arithmetic

**§ 31(1)** Shaped values may define compiler-known arithmetic.

**§ 31(2)** Ordinary `*` remains scalar or elementwise multiplication according to shaped
type rules.

**§ 31(3)** Contextual `x` is matrix multiplication.

**§ 31(4)** No arbitrary user-defined shaped operator overloading exists in Sec 0.1.

---

## § 32 Matrix multiplication operator `x`

### § 32.1 Lexical status

**§ 32.1(1)** `x` is lexed as an identifier.

**§ 32.1(2)** It becomes matrix multiplication only in a parser-confirmed infix expression
context with compatible shaped operands.

**§ 32.1(3)** This permits ordinary identifiers named `x`.

```sec
let x := 10
```

**§ 32.1(4)** and matrix multiplication:

```sec
let result := left x right
```

### § 32.2 Precedence

**§ 32.2(1)** `x` has the same precedence as:

```text
*
/
%
```

**§ 32.2(2)** It associates left.

```sec
a x b x c
```

**§ 32.2(3)** means:

```sec
(a x b) x c
```

**§ 32.2(4)** when shapes are valid.

### § 32.3 Sec 0.1 operand forms

**§ 32.3(1)** Initial supported forms are defined by shaped-type rules and include:

```text
matrix x matrix
matrix x vector
```

**§ 32.3(2)** Other forms require explicit rules.

### § 32.4 Shape validation

**§ 32.4(1)** The inner dimensions must be compatible.

**§ 32.4(2)** Example:

```text
Matrix[M, K] x Matrix[K, N]
    -> Matrix[M, N]
```

**§ 32.4(3)** Shape mismatch is a compile-time error when dimensions are known.

**§ 32.4(4)** Runtime-shaped forms require deterministic validation.

### § 32.5 Formatter

**§ 32.5(1)** Canonical spacing:

```sec
left x right
```

**§ 32.5(2)** The formatter must distinguish infix `x` from an identifier.

---

## § 33 Bitwise operators

**§ 33(1)** Bitwise operators are:

```text
&
|
^
~
```

**§ 33(2)** Operands must be integer types.

**§ 33(3)** Binary operands must be compatible.

**§ 33(4)** Result type is the resolved integer operand type.

**§ 33(5)** Bitwise operations use the fixed-width bit representation of the type.

**§ 33(6)** Extracting the low `N` bits as a narrower unsigned value is the separate compiler-known expression `bit[N](value)` (`types.md` "Explicit low-bit extraction"), not an infix bitwise operator.

**§ 33(7)** They do not perform boolean operations.

**§ 33(8)** Invalid:

```sec
true & false
```

**§ 33(9)** Use:

```sec
true && false
```

---

## § 34 Bitwise AND

```sec
left & right
```

**§ 34(1)** Each result bit is one only when both corresponding operand bits are one.

---

## § 35 Bitwise OR

```sec
left | right
```

**§ 35(1)** Each result bit is one when either corresponding operand bit is one.

---

## § 36 Bitwise XOR

```sec
left ^ right
```

**§ 36(1)** Each result bit is one when corresponding operand bits differ.

---

## § 37 Shift operators

**§ 37(1)** Shift operators are:

```text
<<
>>
```

**§ 37(2)** Both operands must be integers.

**§ 37(3)** The result type is the left operand's type.

---

## § 38 Shift count

**§ 38(1)** A shift count is valid only when:

```text
0 <= count < bit width of left operand
```

**§ 38(2)** Invalid counts include:

```text
negative count
count equal to bit width
count greater than bit width
```

**§ 38(3)** When invalidity is compile-time known, compilation fails.

**§ 38(4)** Otherwise the program performs a deterministic arithmetic failure before the
target shift instruction.

**§ 38(5)** Sec does not inherit target-specific count masking.

**§ 38(6)** Example:

```text
32-bit value << 35
```

**§ 38(7)** does not silently become:

```text
value << 3
```

---

## § 39 Left shift

### § 39.1 Unsigned left shift

**§ 39.1(1)** Unsigned `<<` performs a fixed-width bit shift.

**§ 39.1(2)** Bits shifted beyond the representation are discarded.

**§ 39.1(3)** Zeros enter from the right.

**§ 39.1(4)** This is defined bit behavior, not checked arithmetic overflow.

### § 39.2 Signed left shift

**§ 39.2(1)** Signed `<<` is checked.

**§ 39.2(2)** It is valid only when the mathematical shifted result is representable in the
signed left operand type.

**§ 39.2(3)** Otherwise:

```text
compile-time error
    when known

deterministic arithmetic failure
    at runtime
```

**§ 39.2(4)** The compiler may lower directly to a target shift instruction after proving or
checking validity.

---

## § 40 Right shift

### § 40.1 Unsigned right shift

**§ 40.1(1)** Unsigned `>>` is logical right shift.

**§ 40.1(2)** Zeros enter from the left.

### § 40.2 Signed right shift

**§ 40.2(1)** Signed `>>` is arithmetic right shift.

**§ 40.2(2)** The sign bit is propagated.

**§ 40.2(3)** This behavior is defined by Sec and does not depend on the source target
language or backend default.

---

## § 41 Equality

**§ 41(1)** Equality operators are:

```text
==
!=
```

**§ 41(2)** Result type is `bool`.

**§ 41(3)** Equality requires compatible equality-comparable operands.

**§ 41(4)** No chained equality exists.

---

## § 42 Boolean equality

```sec
left == right
left != right
```

**§ 42(1)** is valid for `bool`.

---

## § 43 Numeric equality

**§ 43(1)** Compatible numeric values may be compared.

**§ 43(2)** Untyped literals may shape into the compatible numeric type.

**§ 43(3)** Unrelated nominal types do not become comparable merely because they share a
representation.

---

## § 44 Floating equality

**§ 44(1)** Floating equality follows these rules:

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

**§ 44(2)** A comparison involving NaN follows IEEE-style unordered semantics.

**§ 44(3)** A future total-order operation for sorting is explicit and not `==`.

---

## § 45 Character and rune equality

**§ 45(1)** `char` compares with `char`.

**§ 45(2)** `rune` compares with `rune`.

**§ 45(3)** No implicit integer comparison exists.

**§ 45(4)** Valid:

```sec
if ch == 0r {
}
```

**§ 45(5)** Valid:

```sec
if character == 0t {
}
```

**§ 45(6)** Invalid:

```sec
if ch == 0 {
}
```

**§ 45(7)** A character literal may be context-shaped according to literal rules.

**§ 45(8)** Example:

```sec
if ch == '$' {
}
```

**§ 45(9)** when `ch` is a `rune`.

**§ 45(10)** A `char` variable and `rune` variable do not implicitly compare with one
another.

**§ 45(11)** Explicit conversion is required when valid.

---

## § 46 String equality

**§ 46(1)** String equality compares exact immutable string content.

**§ 46(2)** It does not compare:

```text
storage address
allocation identity
descriptor identity
padding
reference identity
```

**§ 46(3)** No locale, normalization, or case folding is performed.

---

## § 47 Enum equality

**§ 47(1)** Values of the same enum type support:

```text
==
!=
```

**§ 47(2)** Aliases compare by resolved enum value.

**§ 47(3)** Different enum types do not compare implicitly even when their underlying
integer representation is the same.

**§ 47(4)** Enums do not derive ordered comparison in Sec 0.1.

**§ 47(5)** The unit metadata enums (`UnitKind`, `UnitSystem`, `UnitOrigin`,
`UnitTransform`, `UnitCategory`, `UnitStatus`) and user enums read through
`<U>.Kind`, `<U>.System` or `<U>.Origin` follow this rule unchanged: a member
of one enum never compares with a same-spelled member of another (MD-062).

---

## § 48 Unit dimension equality

**§ 48(1)** `UnitDimension` (`rules/library/core-library.md` § 27.5) supports exactly:

```text
==
!=
```

**§ 48(2)** on two `UnitDimension` operands, with `bool` results. These are
compiler-defined operators resolved for this compiler-known nominal type, not
user operator overloads. `a == b` is true iff the two normalized dimensions
have identical axis identities and exponents; `!=` is its negation. Ordered
comparison and arithmetic on `UnitDimension` are errors. The exact semantics
and runtime requirements are in `rules/types/units.md`, "Unit metadata value
types" (7) (MD-062, 2026-10-27).

---

## § 49 Array equality

**§ 49(1)** Fixed arrays support structural equality when their element type is
equality-comparable.

**§ 49(2)** Comparison proceeds element by element in index order.

**§ 49(3)** It short-circuits at the first unequal element.

**§ 49(4)** Array equality does not compare padding or storage address.

---

## § 50 Slice equality

**§ 50(1)** Slices do not support `==` or `!=` as content comparison in Sec 0.1.

**§ 50(2)** A slice is a view.

**§ 50(3)** Content comparison uses an explicit operation.

**§ 50(4)** Reference or descriptor identity comparison is not exposed through ordinary
slice equality.

---

## § 51 Struct equality

**§ 51(1)** A struct derives structural equality when:

- every stored field is equality-comparable;
- the type has no explicit identity semantics;
- the type has no opaque-resource semantics;
- the type is not explicitly non-comparable;
- equality does not violate a custom invariant.

**§ 51(2)** A normal `impl` block does not disable derived equality.

**§ 51(3)** Example:

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

**§ 51(4)** `Point` may still support:

```sec
first == second
```

**§ 51(5)** Derived comparison:

- processes stored fields in declaration order;
- uses each field's normal equality;
- short-circuits on the first unequal field;
- does not read padding;
- does not compare storage address;
- does not compare methods, properties, or type metadata.

**§ 51(6)** Types that normally do not derive equality include:

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

**§ 51(7)** A custom `free` operation is a strong reason to require an explicit equality
decision.

**§ 51(8)** The exact declaration syntax for identity or non-comparable semantics belongs to
type and attribute rules.

---

## § 52 Union equality

**§ 52(1)** A union supports equality only when its union rules guarantee a safe comparison.

**§ 52(2)** Typical derived behavior:

1. compare active tags;
2. if tags differ, result is unequal;
3. compare the active payload using payload equality.

**§ 52(3)** All possible compared payloads must be equality-comparable.

**§ 52(4)** Inactive payload storage and padding are not compared.

---

## § 53 Reference equality

**§ 53(1)** Safe references do not expose general referent-address equality in Sec 0.1
unless a specialized reference rule explicitly provides it.

**§ 53(2)** Content equality compares dereferenced values explicitly where valid.

**§ 53(3)** This avoids confusing:

```text
same storage
same value
same owner
same resource
```

**§ 53(4)** `reference_model.md` is the specialized reference rule for Sec 0.1. It defines
ordinary equality for compatible safe references as comparison of live storage
identity and referenced location. This does not add equality for `ref T[]` or
`ref mut T[]`, which are slice views under the array/slice rules.

**§ 53(5)** Other identity queries remain explicit APIs rather than additional ordinary
operators.

---

## § 54 Raw pointer equality

**§ 54(1)** `RawPtr[T]` may support address-value equality in unsafe or low-level contexts
according to `raw_pointers.md`.

**§ 54(2)** Such equality compares address representation only.

**§ 54(3)** It does not prove:

```text
same active object
same provenance
same allocation
same lifetime
safe dereference
```

---

## § 55 Interface equality

**§ 55(1)** Direct equality of general interface values is not part of Sec 0.1.

**§ 55(2)** The compiler must first have a complete model for:

```text
borrowed erased values
owned erased values
dynamic type identity
dynamic equality
move-only payloads
opaque resources
```

**§ 55(3)** Concrete values may be compared before erasure.

---

## § 56 Ordered comparison

**§ 56(1)** Ordered comparison operators are:

```text
<
<=
>
>=
```

**§ 56(2)** Result type is `bool`.

**§ 56(3)** They are valid only for approved ordered types.

**§ 56(4)** No chained ordered comparison exists.

---

## § 57 Numeric ordering

**§ 57(1)** Compatible numeric values support ordered comparison.

**§ 57(2)** Integer and decimal ordering use mathematical value order.

**§ 57(3)** Floating ordering follows floating-point comparison rules.

---

## § 58 Floating ordering

**§ 58(1)** For any NaN operand:

```text
left < right
left <= right
left > right
left >= right
```

**§ 58(2)** all evaluate to `false`.

**§ 58(3)** Positive and negative zero compare equal and neither is ordered before the
other through ordinary comparison.

**§ 58(4)** A deterministic total order for sorting NaN payloads, signed zero, and bit
patterns must use an explicit operation.

---

## § 59 Character ordering

**§ 59(1)** `char` values order by unsigned 8-bit value (`0..255`).

**§ 59(2)** No locale or alphabetic collation is applied.

---

## § 60 Rune ordering

**§ 60(1)** `rune` values order by Unicode scalar value.

**§ 60(2)** No locale, normalization, or case folding is applied.

**§ 60(3)** `==`, `!=`, `<`, `<=`, `>` and `>=` apply to two operands of the **same**
character type (`char` with `char`, `rune` with `rune`); there is no implicit
conversion between `char` and `rune` or between characters and integers, so
`c < r` is an error. Both comparisons are unsigned: `char` by its 8-bit value,
`rune` by its scalar value. They lower through the integer comparison IR —
Semantic IR `int.compare` and Sec MLIR `sec.int.cmp` over `!sec.char` /
`!sec.rune` (`char -> ui8`, `rune -> ui32`) — with no separate character
operation (`rules/compiler/semantic_ir.md` § 125; MD-105, 2026-10-18).

---

## § 61 String ordering

**§ 61(1)** Strings order lexicographically by Unicode scalar sequence.

**§ 61(2)** Comparison proceeds from the first scalar value.

**§ 61(3)** At the first difference, scalar-value ordering determines the result.

**§ 61(4)** When one string is an exact prefix of the other, the shorter string orders
first.

**§ 61(5)** No operation is performed for:

```text
locale collation
Swedish alphabetic ordering
case folding
Unicode normalization
natural-number ordering
```

**§ 61(6)** Those require explicit library operations.

---

## § 62 Types without ordering

**§ 62(1)** Ordinary ordered comparison is not provided for:

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

**§ 62(2)** unless a specialized canonical rule explicitly adds it.

---

## § 63 Membership operators `in` and `not in`

**§ 63(1)** `in` has two different grammar roles. The two-token spelling `not in` exists
only in membership-expression position and is not iteration grammar.

**§ 63(2)** They must not be conflated.

---

## § 64 Membership expression

**§ 64(1)** Expression form:

```sec
value in collection
value not in collection
```

**§ 64(2)** returns `bool`.

**§ 64(3)** `value not in collection` is the logical complement of
`value in collection`. It preserves the same operand evaluation order,
single-evaluation guarantees, short-circuit search, type compatibility,
ownership behavior, allocation behavior, and diagnostics. The membership
result is negated only after the search has completed or short-circuited.

**§ 64(4)** `not in` is one compound contextual operator at the same precedence as `in`.
The lexer may emit ordinary identifier `not` followed by keyword `in`; the
parser must retain the canonical AST operator spelling `not in`. A standalone
identifier named `not` remains legal outside this infix context.

**§ 64(5)** Sec 0.1 supports membership for:

```text
contextual range
fixed array
owning dynamic array
slice
```

**§ 64(6)** It does not use arbitrary method lookup.

---

## § 65 Range membership

```sec
value in lower..<upper
```

**§ 65(1)** or:

```sec
value in lower..upper
```

**§ 65(2)** uses the range's inclusive or exclusive bound semantics.

**§ 65(3)** Bounds evaluate left to right.

**§ 65(4)** The value evaluates before the right range expression.

**§ 65(5)** Range membership uses compatible ordered values.

---

## § 66 Fixed-array membership

```sec
value in values
```

**§ 66(1)** when `values` is a fixed array:

- evaluates `value` first;
- evaluates the array expression once;
- examines elements from index zero upward;
- uses element equality;
- stops at the first match;
- returns `true` on match;
- returns `false` after all elements differ;
- performs no allocation.

**§ 66(2)** The array element type must be equality-comparable.

**§ 66(3)** The left value must be compatible with the element type.

---

## § 67 Dynamic-array membership

**§ 67(1)** Membership in an owning dynamic array (`T[]`) follows fixed-array membership
semantics. The search evaluates the left value first and the array expression
once, visits initialized elements from index zero up to the current length,
and stops at the first equal element. An empty array yields `false`.

**§ 67(2)** The search borrows the array for observation; it neither consumes the array nor
its elements, allocates, nor structurally mutates storage. Element equality and
value compatibility follow the same rules as fixed arrays and slices. Spare
capacity is not searched. Creation of the array expression retains its own
allocation and effect requirements.

---

## § 68 Slice membership

**§ 68(1)** Slice membership follows the same content semantics as array membership.

```sec
value in view
```

- evaluates `value` first;
- evaluates the slice expression once;
- reads elements left to right;
- stops at first match;
- performs no allocation;
- does not structurally mutate the slice or backing storage.

**§ 68(2)** The slice and references must remain valid during the operation.

---

## § 69 Unsupported membership in Sec 0.1

**§ 69(1)** The `in` and `not in` operators are not defined for:

```text
list
set
map
string
iterator
generator
arbitrary user type
```

**§ 69(2)** Use explicit APIs.

**§ 69(3)** Examples:

```sec
values.Contains(item)
map.ContainsKey(key)
text.Contains(substring)
```

**§ 69(4)** This may be extended in a future language version.

---

## § 70 `for ... in ...`

**§ 70(1)** Iteration syntax:

```sec
for value in values {
}
```

**§ 70(2)** is not the membership operator.

**§ 70(3)** It is part of the `for` grammar.

**§ 70(4)** It:

- obtains an iteration source;
- binds each produced element;
- follows iterator ownership and borrow rules;
- does not return `bool`;
- does not perform a membership search.

**§ 70(5)** The parser distinguishes the forms by grammar context.
`for value not in values` is not valid iteration syntax.

---

## § 71 Ranges

**§ 71(1)** Ranges are contextual syntax in Sec 0.1.

**§ 71(2)** Valid uses include:

```sec
for i in 0..<10 {
}

if value in 0..100 {
}

let view := values[2..<8]

let lower := [224r..246r, 248r..255r]

try values.Append(0r..127r)
```

**§ 71(3)** Array literal range segments and the `Append` argument are defined by
`rules/collections/collections.md` §§ 5.6a and 6.7.

**§ 71(4)** A range is not a general first-class runtime value in Sec 0.1.

**§ 71(5)** Invalid:

```sec
let range := 0..<10
```

**§ 71(6)** No general `Range[T]` variable type is implied.

---

## § 72 Inclusive range

```sec
start..end
```

**§ 72(1)** includes both endpoints where the context permits.

---

## § 73 Exclusive upper range

```sec
start..<end
```

**§ 73(1)** includes `start` and excludes `end`.

**§ 73(2)** This is the canonical half-open form for indexing and iteration.

---

## § 74 Range operands

**§ 74(1)** Range operand types and step behavior belong to iteration, slicing, and range
rules.

**§ 74(2)** The parser must not accept chained ranges as one expression.

**§ 74(3)** Invalid:

```sec
a..<b..<c
```

---

## § 75 Logical operators

**§ 75(1)** Logical operators are:

```text
&&
||
!
```

**§ 75(2)** All operands must be `bool`.

**§ 75(3)** Sec does not use truthy or falsy conversions.

---

## § 76 Assignment statements

**§ 76(1)** Assignment is statement syntax.

**§ 76(2)** It does not produce a value.

**§ 76(3)** Invalid:

```sec
let result := destination = source
```

**§ 76(4)** Invalid:

```sec
a = b = c
```

**§ 76(5)** Assignments do not associate.

---

## § 77 Ordinary declaration initialization

```sec
let destination := source
```

**§ 77(1)** `:=` is ordinary inferred initialization.

**§ 77(2)** From an existing reusable place, it requires copyability.

**§ 77(3)** It never silently moves a move-only value.

**§ 77(4)** Fresh temporaries may initialize directly.

---

## § 78 Move declaration initialization

```sec
let destination :<- source
```

**§ 78(1)** `:<-` explicitly moves from an existing source place and infers destination
type.

**§ 78(2)** The source becomes unavailable.

---

## § 79 Typed ordinary initialization

```sec
let destination: Type := source
```

**§ 79(1)** Ordinary copy or direct construction semantics apply.

---

## § 80 Typed move initialization

```sec
let destination: Type <- source
```

**§ 80(1)** The explicit type uses `:`.

**§ 80(2)** The transfer operator is `<-`.

---

## § 81 Ordinary assignment

```sec
destination = source
```

**§ 81(1)** This is ordinary assignment, replacement, or reinitialization.

**§ 81(2)** From an existing reusable source place, copyability is required.

**§ 81(3)** It never silently moves.

---

## § 82 Move assignment

```sec
destination <- source
```

**§ 82(1)** This explicitly transfers the source value or ownership responsibility.

**§ 82(2)** The source becomes unavailable.

---

## § 83 Assignment evaluation order

**§ 83(1)** For:

```sec
destination = source
```

**§ 83(2)** conceptual order:

1. evaluate destination place exactly once;
2. evaluate source expression;
3. validate conversion, contracts, ownership, and borrows;
4. preserve old destination until commit is safe;
5. destroy old destination when replacement requires it;
6. install the new value.

**§ 83(3)** Move assignment additionally marks source unavailable only after valid commit.

---

## § 84 Compound assignment

**§ 84(1)** Compound assignment operators are:

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

**§ 84(2)** Conceptually:

```sec
destination op= source
```

**§ 84(3)** means:

```text
read destination once
evaluate source once
apply operator
validate result
write destination once
```

**§ 84(4)** It is not defined as textual rewriting.

---

## § 85 Compound assignment target

**§ 85(1)** The target must be a writable mutable place.

**§ 85(2)** Possible targets include:

```text
mutable local
mutable field
mutable property with setter
mutable indexed place
mutable dereferenced reference (`*r` for `r: ref mut T`)
compiler-known writable location
```

**§ 85(3)** Exact validity depends on the target type and place rules.

---

## § 86 Compound assignment and contracts

**§ 86(1)** When the destination type has a fallible contract, compound assignment requires
the canonical `try` form.

**§ 86(2)** Example:

```sec
try percent += Percent(i) {
    Err(error) => {
        discard error
    }
}
```

**§ 86(3)** The old destination remains valid until the new result passes validation.

---

## § 87 Compound arithmetic failure

**§ 87(1)** Arithmetic failure from a compound operator follows the underlying operator.

**§ 87(2)** Examples:

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

**§ 87(3)** No destination mutation commits before successful validation.

---

## § 88 Increment and decrement aliases

**§ 88(1)** Sec accepts statement-only:

```sec
value++
value--
```

**§ 88(2)** They have no expression result.

**§ 88(3)** The formatter normalizes:

```sec
value++
```

**§ 88(4)** to:

```sec
value += 1
```

**§ 88(5)** and:

```sec
value--
```

**§ 88(6)** to:

```sec
value -= 1
```

**§ 88(7)** Invalid:

```sec
let previous := value++
```

**§ 88(8)** The resulting compound assignment still follows:

```text
mutability
overflow
contracts
try
ownership
```

---

## § 89 Self-assignment

**§ 89(1)** Ordinary copy self-assignment:

```sec
value = value
```

**§ 89(2)** may be accepted for copyable values and optimized to no operation.

**§ 89(3)** The compiler may emit an advisory diagnostic.

**§ 89(4)** Move self-assignment is invalid:

```sec
value <- value
```

---

## § 90 Overlapping places

**§ 90(1)** Assignment and move operations must validate place overlap.

**§ 90(2)** Potentially invalid:

```sec
object.Field <- object
```

**§ 90(3)** when source and destination overlap incompatibly.

**§ 90(4)** The canonical place model from `memory_model.md` determines:

```text
same
disjoint
contains
contained by
potentially overlapping
unknown
```

---

## § 91 `ref`, `try`, `await`, and `spawn`

**§ 91(1)** These are parser-defined prefix expression forms or statement forms.

**§ 91(2)** They are not ordinary symbolic operators available for user overloading.

**§ 91(3)** Their detailed semantics belong to:

```text
references and borrowing rules
error and try rules
tasks and await rules
spawn rules
```

**§ 91(4)** They occupy prefix parsing positions where applicable.

---

## § 92 Question mark `?`

**§ 92(1)** `?` is lexed and reserved.

**§ 92(2)** It has no language meaning in Sec 0.1.

**§ 92(3)** The parser should emit a focused diagnostic:

```text
`?` is reserved and has no meaning in Sec 0.1
```

**§ 92(4)** Its future use remains open.

**§ 92(5)** Possible future designs may include conditionals or another language feature.

**§ 92(6)** Sec 0.1 does not assign Rust-style error propagation or any other meaning.

---

## § 93 Unsupported operators

**§ 93(1)** Sec 0.1 does not define:

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

**§ 93(2)** A future language version may define new syntax through a separate rule.

---

## § 94 Operator failure classes

**§ 94(1)** An operator may be:

```text
infallible
compile-time rejected
runtime checked
fallible through Result or try
short-circuiting
target-restricted
unsafe
```

**§ 94(2)** The resolved operator must record its class.

---

## § 95 Deterministic arithmetic failure

**§ 95(1)** Operations such as checked integer overflow and runtime integer division by zero
require a deterministic arithmetic-failure path.

**§ 95(2)** The exact representation may depend on the surrounding expression and profile.

**§ 95(3)** It must not become:

```text
undefined behavior
silent wrapping
random target trap semantics
optimizer-dependent behavior
```

**§ 95(4)** A future arithmetic-error rulebook may define:

```text
Result integration
panic integration
checked expression syntax
target trap lowering
no-runtime lowering
```

**§ 95(5)** Until then, the compiler must preserve a distinguishable failure edge.

---

## § 96 Constant folding

**§ 96(1)** The compiler may evaluate operators at compile time when operands are constant.

**§ 96(2)** Constant folding must use Sec semantics.

**§ 96(3)** It must not use host-language behavior when that differs.

**§ 96(4)** Examples include:

```text
checked overflow
truncation-toward-zero division
float NaN comparison
unit conversion
string literal concatenation
shift-count validation
```

**§ 96(5)** A constant expression that would deterministically fail is a compile-time error,
except for checked integer overflow covered by `S1023`. That case is a warning
and retains checked runtime failure semantics.

---

## § 97 Untyped literals

**§ 97(1)** Untyped literals may be shaped by operator context.

**§ 97(2)** Examples:

```sec
let value: int32 := 10
let result := value + 1
```

**§ 97(3)** The literal may become `int32`.

**§ 97(4)** Literal shaping must not introduce:

```text
overflow
lossy conversion
nominal type confusion
char/rune integer comparison
unit mismatch
```

---

## § 98 Named types

**§ 98(1)** A named type retains nominal identity.

**§ 98(2)** An operator may be available when:

- the named type's canonical rule permits it;
- operands are the same compatible named type;
- a literal shapes directly into that type;
- an explicit conversion has occurred;
- a unit rule defines compatible algebra.

**§ 98(3)** Underlying representation alone does not grant every base-type operator.

---

## § 99 Contracts

**§ 99(1)** Operator results assigned to constrained types must satisfy contracts.

**§ 99(2)** A contract may be:

```text
compile-time proven
runtime checked
fallible
```

**§ 99(3)** Operators do not bypass contracts.

**§ 99(4)** A same-type move preserves an already valid value.

**§ 99(5)** Arithmetic producing a new value requires normal validation.

---

## § 100 Enums

**§ 100(1)** Enums support:

```text
==
!=
```

**§ 100(2)** according to enum rules.

**§ 100(3)** Enums do not automatically support:

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

**§ 100(4)** even when the underlying representation is integer.

**§ 100(5)** Any future flags-enum model requires a separate rule.

---

## § 101 Registers

**§ 101(1)** Register expressions may support compiler-known bitwise and numeric operations on
snapshots.

**§ 101(2)** Access to addressed register storage follows:

```text
volatile rules
exact access-width rules
reserved-bit rules
fixed-address rules
target rules
```

**§ 101(3)** An operator on a local snapshot is not itself a hardware write.

**§ 101(4)** Compound assignment to a register-backed property may lower to a defined
read-modify-write operation only when register rules permit it.

---

## § 102 Raw pointers

**§ 102(1)** Raw pointer arithmetic, comparison, and difference are not granted by ordinary
numeric operator rules.

**§ 102(2)** Any supported raw-pointer operator must be:

```text
explicitly defined
unsafe where required
provenance-aware
bounds-aware where known
target-representable
```

**§ 102(3)** Address-value equality may be permitted by `raw_pointers.md`.

**§ 102(4)** Ordered raw-pointer comparison is not part of ordinary Sec 0.1 operators.

---

## § 103 Atomics

**§ 103(1)** Ordinary operators on atomic storage are not automatically atomic
read-modify-write operations.

**§ 103(2)** Atomic types expose compiler-known methods or explicit atomic operations with
memory ordering.

**§ 103(3)** The source:

```sec
atomicValue += 1
```

**§ 103(4)** must not silently become an atomic fetch-add unless an atomic rule explicitly
defines that syntax.

---

## § 104 Volatile values

**§ 104(1)** Ordinary arithmetic on a local value loaded from volatile storage is ordinary
arithmetic.

**§ 104(2)** Accessing volatile storage must remain an explicit volatile load or store in
Semantic IR.

**§ 104(3)** The compiler must not duplicate or eliminate volatile accesses while applying
operator transformations.

**§ 104(4)** Compound assignment to volatile storage requires one defined read and one
defined write unless a register rule says otherwise.

---

## § 105 Formatter rules

**§ 105(1)** Canonical spacing:

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

**§ 105(2)** Prefix operators have no following space:

```sec
-value
+value
!value
~value
```

**§ 105(3)** Member access has no surrounding spaces:

```sec
value.Member
```

**§ 105(4)** Calls and indexes have no preceding space:

```sec
Call(value)
values[index]
```

---

## § 106 Formatter normalization

**§ 106(1)** Ordinary formatter may normalize parser-confirmed:

```text
func -> fn
x++ -> x += 1
x-- -> x -= 1
```

**§ 106(2)** It must preserve:

```text
:= versus :<-
= versus <-
```

**§ 106(3)** It must not infer semantic move syntax.

**§ 106(4)** It must recognize contextual `x` before formatting it as an infix operator.

**§ 106(5)** An identifier named `x` remains an identifier.

---

## § 107 Parser recovery

**§ 107(1)** The parser should recover from operator errors without discarding the complete
surrounding expression.

**§ 107(2)** Examples:

```text
missing right operand
missing left operand
duplicated operator
invalid chained comparison
assignment in expression context
reserved question mark
missing closing delimiter
```

**§ 107(3)** Recovery nodes should preserve:

```text
operator token
known operand
missing operand position
expected category
source range
```

**§ 107(4)** This supports LSP diagnostics and fixes.

---

## § 108 Diagnostics

**§ 108(1)** Operator diagnostics require stable IDs.

**§ 108(2)** Suggested rules:

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

**§ 108(3)** The current registry assigns these IDs to the implemented subset:

```text
S1016  operator.non-orderable-operands
S1017  operator.invalid-shift-count
S1018  operator.signed-left-shift-overflow
S1019  operator.non-comparable-operands
S1021  operator.invalid-membership
S1022  operator.invalid-concat-operand
S1029  operator.invalid-interpolation-value
```

**§ 108(4)** Retired and permanently reserved:

```text
S1020  operator.string-runtime-concat
```

**§ 108(5)** Stable IDs for the remaining concat categories must be assigned through the
diagnostic registry when their checks are implemented.

**§ 108(6)** Safety errors are mandatory.

**§ 108(7)** Performance findings may be advisory.

---

## § 109 Diagnostic examples

### § 109.1 Invalid concatenation operand

```text
error[S1022]: `int` cannot be concatenated directly with `string`
string concatenation accepts string, char, and rune
help: use `value.ToString()` inside the concatenation with `try` before the whole expression, or interpolation when a formatting contract exists
```

### § 109.2 Invalid interpolation value

```text
error[S1029]: `Packet` has no canonical interpolation formatting contract
help: define an exact shared `fn ToString() Result[string, StringError]` method or interpolate a supported printable value
```

### § 109.3 Invalid rune comparison

```text
error[S....]: rune cannot be compared with int
help: use `0r` for rune zero or convert explicitly
```

### § 109.4 Invalid char comparison

```text
error[S....]: char cannot be compared with int
help: use `0t` for char zero or convert explicitly
```

### § 109.5 Chained comparison

```text
error[P....]: comparisons cannot be chained
help: write `a < b && b < c`
```

### § 109.6 Invalid shift count

```text
error[S....]: shift count 32 is outside the valid range 0..<32
```

### § 109.7 Invalid membership

```text
error[S....]: `in` supports ranges, fixed arrays, dynamic arrays, and slices in Sec 0.1
```

### § 109.8 Missing concatenation allocation context

```text
error[S....]: runtime string concatenation requires an allocation context
```

### § 109.9 Concatenation in `@noPanic`

```text
error[S....]: runtime concatenation without `try` may panic on allocation failure
help: write `try` and handle or propagate StringError
```

---

## § 110 LSP integration

**§ 110(1)** The LSP should expose at an operator:

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

**§ 110(2)** Example hover:

```text
Operator: signed integer addition
Type: int32
Overflow: checked
Evaluation: left operand, then right operand
Result: int32
```

**§ 110(3)** Example for `&&`:

```text
Operator: lazy logical AND
Right operand executes only when left is true
```

**§ 110(4)** Example for `x`:

```text
Operator: matrix multiplication
Left: Matrix[4, 3, float32]
Right: Matrix[3, 2, float32]
Result: Matrix[4, 2, float32]
```

**§ 110(5)** Example for runtime string `+`:

```text
Operator: string concatenation
Result: string
May allocate: yes
Allocation context: current arena
Failure policy: panic, or StringError through try
Concat plan: maximal chain
```

---

## § 111 Code actions

**§ 111(1)** Possible operator code actions include:

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

**§ 111(2)** A fix must be machine-applicable only when intent is unique.

---

## § 112 Semantic IR

**§ 112(1)** Every resolved operator must lower to an explicit semantic operation.

**§ 112(2)** Conceptual operations include:

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

**§ 112(3)** Exact operation names may differ.

**§ 112(4)** The semantic distinctions must remain explicit.

---

## § 113 Semantic IR operator metadata

**§ 113(1)** Where relevant, record:

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

## § 114 Semantic IR verification

**§ 114(1)** Before MLIR lowering:

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

**§ 114(2)** Failure is an internal compiler error after successful Sema.

---

## § 115 MLIR lowering

**§ 115(1)** MLIR may use:

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

**§ 115(2)** The selected lowering must preserve Sec semantics.

**§ 115(3)** Examples:

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

## § 116 LLVM and backend requirements

**§ 116(1)** Backends must not rely on undefined target-language behavior for Sec checked
operations.

**§ 116(2)** They must preserve:

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

## § 117 Float comparison lowering requirement

**§ 117(1)** Ordinary float `!=` must be true when either operand is NaN.

**§ 117(2)** A backend predicate equivalent to ordered-not-equal is insufficient.

**§ 117(3)** The lowering must implement unordered-not-equal semantics for Sec `!=`.

**§ 117(4)** Ordered comparisons remain false with NaN.

---

## § 118 Optimization

**§ 118(1)** The compiler may optimize operators when results and observable behavior remain
equivalent.

**§ 118(2)** Allowed examples:

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

**§ 118(3)** The compiler must not optimize away:

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

## § 119 Constant-expression rules

**§ 119(1)** Operators usable in constant expressions must:

```text
be deterministic
have compile-time-known operands
not require runtime allocation
not perform I/O
not access volatile memory
not depend on runtime target state
```

**§ 119(2)** Compile-time string literal concatenation is allowed.

**§ 119(3)** It folds to one string constant and requires no runtime allocation context,
`try`, or allocation-failure policy.

**§ 119(4)** Compile-time arithmetic uses Sec checked semantics.

---

## § 120 Testing

### § 120.1 String concatenation tests

**§ 120.1(1)** Test all nine ordered pairs of `string`, `char`, and `rune`; every result must
be `string`. Reject direct integer, float, decimal, bool, enum, struct, union,
array, slice, collection, interface, and user-defined nominal operands with
`S1022`, both operand types, the accepted text categories, and explicit
conversion/interpolation help.

**§ 120.1(2)** Test compile-time folding, interpolation formatting contracts, mixed chains,
left-to-right exact-once evaluation, active allocation contexts, panic and
`try` policies, `@noPanic`, checked total length, one maximal concat plan, and
at-most-once canonical result allocation.

**§ 120.1(3)** For `string +=`, test exact-once destination and source evaluation, both failure
modes, the protected and separately protected `Result[string, StringError]`
operand, rejection of an unhandled one, transactional failure, unchanged
destination after failed `try`, and semantic equivalence under proven in-place
optimization. Test that structural
`StringBuilder` information appears only for repeated reconstruction and never
for one isolated operation or one maximal chain.

**§ 120.1(4)** Verify that valid runtime concatenation never emits retired `S1020`, that the
ID is not reused, and that every replacement diagnostic has a stable registry
entry before it is emitted.

### § 120.2 Lexer tests

**§ 120.2(1)** Test every operator token and longest-match conflict.

**§ 120.2(2)** Required examples include:

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

### § 120.3 Parser precedence tests

**§ 120.3(1)** Test every adjacent precedence pair.

**§ 120.3(2)** Examples:

```sec
a + b * c
a << b + c
a == b & c
a & b == c
a || b && c
a x b + c
value.Member()[index].Field
```

### § 120.4 Associativity tests

```sec
a - b - c
a / b / c
a << b << c
a x b x c
```

**§ 120.4(1)** Verify comparisons do not chain.

### § 120.5 Evaluation-order tests

**§ 120.5(1)** Use observable test helpers to verify:

```text
binary operands
call arguments
index base and index
struct fields
array elements
spread
compound assignment target
```

### § 120.6 Arithmetic tests

**§ 120.6(1)** Test every integer width and signedness for:

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

### § 120.7 Float tests

**§ 120.7(1)** Test:

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

### § 120.8 Decimal tests

**§ 120.8(1)** Test:

```text
exact remainder
negative operands
zero divisor
precision failure
range failure
compound remainder
preferred and fallback result scale for + - * %
exact division without padding (1m / 8m has Scale 3)
rounded division at the finest scale (1m / 3m, 2m / 3m, 1000000000m / 3m)
signed coefficient minimum and rounding carry
division underflow to rounded zero versus exact zero
provably zero divisor is a compile-time error, with and without try
runtime zero divisor: panic versus ArithmeticError.DivisionByZero
remainder with a repeating quotient uses the exact truncated quotient
ScaleOverflow, PrecisionExhausted and Overflow classification
comparison ignores scale (1.2m == 1.20m)
```

**§ 120.8(2)** The complete conformance matrix is DEC-01 to DEC-22 in
`rules/corrections/applied/md066-md067-md068-decimal-correction-20261028.md` § 14.

### § 120.9 Shift tests

**§ 120.9(1)** Test every integer width:

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

### § 120.10 Equality tests

**§ 120.10(1)** Test:

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

### § 120.11 Ordering tests

**§ 120.11(1)** Test:

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

### § 120.12 Membership tests

**§ 120.12(1)** Test:

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

### § 120.13 Assignment tests

**§ 120.13(1)** Test:

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

### § 120.14 Formatter tests

**§ 120.14(1)** Test canonical spacing and:

```text
x identifier versus matrix operator
++ normalization
-- normalization
copy versus move preservation
parentheses
multiline expressions
```

---

## § 121 Required source test files

**§ 121(1)** Create or update:

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

**§ 121(2)** Every invalid test includes:

```sec
/* Expected error: ...
 * Reason: ...
 */
```

---

## § 122 Required synchronization

**§ 122(1)** This rulebook must remain synchronized with:

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
destruction.md
concurrency_memory_model.md
semantic_ir.md
compiler_pipeline.md
diagnostics.md
lsp.md
language-rulebook-status.md
```

**§ 122(2)** Files that do not yet exist remain planned dependencies.

---

## Appendix A — Codex implementation plan

### Appendix A.1 Add the canonical rulebook

**Appendix A.1(1)** Add:

```text
rules/foundations/operators.md
```

**Appendix A.1(2)** Update:

```text
language-rulebook-status.md
governance/sema.yaml (frontend.operators-v2)
```

**Appendix A.1(3)** Mark the document as written.

---

### Appendix A.2 Create one precedence definition

**Appendix A.2(1)** Move precedence metadata into one compiler package shared by parser tests,
formatter, LSP, and documentation generation where practical.

**Appendix A.2(2)** Do not maintain unrelated precedence tables.

**Appendix A.2(3)** The parser may retain efficient constants generated from the canonical table.

---

### Appendix A.3 Add contextual `x`

**Appendix A.3(1)** Parser and fixed-shape Sema support are implemented.

**Appendix A.3(2)** Identifier token `x` becomes an infix operator only when:

- parser is in infix position;
- the surrounding token structure is valid;
- Sema resolves compatible shaped operands.

**Appendix A.3(3)** Preserve ordinary identifiers named `x`.

**Appendix A.3(4)** Formatter and semantic-token context, Semantic IR and backend lowering remain.

---

### Appendix A.4 Enforce strict evaluation order

**Appendix A.4(1)** Audit:

```text
parser AST order
Sema effect order
Semantic IR
direct LLVM backend
MLIR backend
optimizations
```

**Appendix A.4(2)** Represent observable left-to-right order explicitly where needed.

**Appendix A.4(3)** Add exact-once compound-target tests.

---

### Appendix A.5 Preserve short-circuit lowering

**Appendix A.5(1)** Retain current CFG-based implementation for:

```text
&&
||
```

**Appendix A.5(2)** Extend Sema ownership, borrow, effect, and lifetime merging across the right
operand.

**Appendix A.5(3)** Add Semantic IR short-circuit nodes or explicit blocks.

---

### Appendix A.6 Implement checked integer arithmetic

**Appendix A.6(1)** Add checks for:

```text
+
-
*
unary -
signed <<
division minimum / -1
compound forms
```

**Appendix A.6(2)** Compile-time constants use arbitrary-precision evaluation and target type range
checks.

**Appendix A.6(3)** Runtime lowering uses target operations plus explicit failure edges.

**Appendix A.6(4)** Do not rely on LLVM undefined signed overflow.

---

### Appendix A.7 Division and remainder

**Appendix A.7(1)** Implement truncation-toward-zero integer division and remainder.

**Appendix A.7(2)** Add deterministic zero-divisor checks.

**Appendix A.7(3)** Add:

```text
float truncation-based remainder
decimal truncation-based remainder
```

**Appendix A.7(4)** Preserve special float cases.

---

### Appendix A.8 Fix float `!=`

**Appendix A.8(1)** Current ordered-not-equal lowering is incompatible with:

```text
NaN != NaN == true
```

**Appendix A.8(2)** Use unordered-not-equal or equivalent explicit logic.

**Appendix A.8(3)** Verify every float predicate against this rulebook.

---

### Appendix A.9 Implement shift validation

**Appendix A.9(1)** Before lowering:

- prove or check `0 <= count < width`;
- use `ashr` for signed right shift;
- use `lshr` for unsigned right shift;
- use ordinary fixed-width shift for unsigned left;
- check signed left-shift representability.

**Appendix A.9(2)** Do not mask counts silently.

---

### Appendix A.10 Ordered comparison validation

**Appendix A.10(1)** Update Sema so `<`, `<=`, `>`, and `>=` accept only:

```text
compatible numeric values
char with char
rune with rune
string with string
```

**Appendix A.10(2)** Reject enums, arrays, slices, structs, unions, references, and interfaces unless
a future rule adds support.

---

### Appendix A.11 String comparison

**Appendix A.11(1)** Implement exact string equality and Unicode-scalar lexicographic ordering.

**Appendix A.11(2)** Do not add:

```text
locale collation
normalization
case folding
```

**Appendix A.11(3)** Avoid allocation.

---

### Appendix A.12 Struct equality

**Appendix A.12(1)** Implement derived structural equality.

**Appendix A.12(2)** Requirements:

- all stored fields equality-comparable;
- normal `impl` does not disable derivation;
- no padding comparison;
- declaration-order comparison;
- short-circuit first mismatch;
- reject explicit identity/resource/non-comparable types.

**Appendix A.12(3)** Add temporary internal metadata until canonical attribute syntax is defined.

---

### Appendix A.13 Interface equality restriction

**Appendix A.13(1)** Reject direct equality on general interface values in Sec 0.1.

**Appendix A.13(2)** Do not compare erased descriptor bits or addresses.

---

### Appendix A.14 Array and slice membership

**Appendix A.14(1)** Sema implements and validates:

```text
value in fixedArray
value in slice
```

**Appendix A.14(2)** Lower as left-to-right short-circuit search.

**Appendix A.14(3)** Evaluate the left value and collection expression exactly once.

**Appendix A.14(4)** Do not allocate.

**Appendix A.14(5)** Do not use arbitrary `.Contains` method dispatch.

**Appendix A.14(6)** Semantic IR and backend lowering remain pending.

---

### Appendix A.15 Preserve range-only range values

**Appendix A.15(1)** Continue contextual parsing of:

```text
for ranges
membership ranges
slice ranges
```

**Appendix A.15(2)** Reject standalone range values in Sec 0.1.

**Appendix A.15(3)** Add a focused diagnostic.

---

### Appendix A.16 Complete runtime string concatenation

1. Accept the canonical direct operand matrix.
2. Reject non-text operands without hidden conversion.
3. Recognize interpolation as an explicit formatting context.
4. Resolve the active allocation context.
5. Select panic or `StringError` flow from source `try` context.
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

### Appendix A.17 Question mark diagnostic

**Appendix A.17(1)** Keep `QUESTION` token.

**Appendix A.17(2)** Reject it in parser expression contexts with:

```text
`?` is reserved and has no meaning in Sec 0.1
```

**Appendix A.17(3)** Do not assign error propagation, optional access, or ternary semantics.

---

### Appendix A.18 Compound assignments

**Appendix A.18(1)** Audit every compound operator for:

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

**Appendix A.18(2)** Represent compound assignment semantically rather than by AST duplication.

---

### Appendix A.19 Semantic IR

**Appendix A.19(1)** Add resolved operations and metadata from this rulebook.

**Appendix A.19(2)** Ensure no generic operator token reaches MLIR without semantic classification.

---

### Appendix A.20 Formatter

**Appendix A.20(1)** Update `internal/formatter` when created.

**Appendix A.20(2)** Support:

```text
canonical spacing
contextual x
move operators
++ and -- normalization
parentheses from precedence
recoverable expressions
```

**Appendix A.20(3)** Do not duplicate precedence logic.

---

### Appendix A.21 LSP

**Appendix A.21(1)** Expose:

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

**Appendix A.21(2)** Use compiler-owned operator facts.

---

### Appendix A.22 Diagnostics

**Appendix A.22(1)** Register stable IDs.

**Appendix A.22(2)** Attach:

```text
operand ranges
resolved operand types
expected categories
failure cause
safe fixes
related declarations
```

---

### Appendix A.23 Backend audit

**Appendix A.23(1)** Audit direct LLVM and MLIR implementations against the same test matrix.

**Appendix A.23(2)** Do not allow backend feature divergence to become source-language divergence.

**Appendix A.23(3)** A backend may report unsupported lowering.

**Appendix A.23(4)** It may not silently produce different semantics.

---

### Appendix A.24 Recommended implementation order

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
11. Complete runtime concat allocation/effect analysis and consume the
    frontend `StringConcatPlan` in Semantic IR and lowering. (Direct operands,
    interpolation format selection, and maximal frontend planning are complete.)
12. Add question-mark diagnostic.
13. Complete Semantic IR metadata.
14. Synchronize formatter and LSP.
15. Audit all backends.
```

---

## § 123 Design summary

**§ 123(1)** Sec operators have one canonical precedence table.

**§ 123(2)** All ordinary expression operands evaluate strictly left to right.

**§ 123(3)** `&&` and `||` use lazy short-circuit evaluation.

**§ 123(4)** Integer arithmetic is checked.

**§ 123(5)** Integer division truncates toward zero.

**§ 123(6)** `%` uses truncation-based remainder for integers, floats, and decimals.

**§ 123(7)** Shifts use strict counts.

**§ 123(8)** Signed right shift is arithmetic.

**§ 123(9)** Unsigned right shift is logical.

**§ 123(10)** Unsigned left shift discards high bits.

**§ 123(11)** Signed left shift is checked.

**§ 123(12)** `char`, `rune`, and `string` support exact deterministic ordering.

**§ 123(13)** Strings order lexicographically by Unicode scalar sequence.

**§ 123(14)** Structs derive structural equality when their stored fields are comparable and
the type has no identity or opaque-resource semantics.

**§ 123(15)** A normal `impl` does not disable equality.

**§ 123(16)** Ranges remain contextual in Sec 0.1.

**§ 123(17)** `in` supports ranges, fixed arrays, dynamic arrays, and slices.

**§ 123(18)** `for value in source` is iteration grammar, not membership.

**§ 123(19)** String `+` accepts exactly built-in `string`, `char`, and `rune` operands and
always produces `string`; other values require explicit conversion, while
interpolation is an explicit formatting context.

**§ 123(20)** Compile-time concatenation folds without allocation. Runtime concatenation uses
the active allocation context and selects deterministic allocation panic by
default or `StringError` flow through `try`. Maximal concatenation,
interpolation, `string.Concat`, and transactional `string +=` share one
`StringConcatPlan` before MLIR.

**§ 123(21)** `?` remains reserved with no Sec 0.1 meaning.

**§ 123(22)** Arbitrary user-defined operator declarations are not supported.

**§ 123(23)** Sema resolves every operator completely.

**§ 123(24)** Semantic IR preserves the resolved operation.

**§ 123(25)** MLIR and backends implement Sec semantics without changing them.
