# Type Contracts

- **Status:** Normative
- **Created:** 2026-08-13
- **Last updated:** 2026-08-13
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/types/contracts.md`
- **Implementation governance:** `governance/types.yaml`
- **Replaces:** `rules/types/variable-contracts.txt`

---

## Status

This is the canonical Sec rulebook for type contracts. Its canonical filename is
`contracts.md`; the obsolete variable-contract rulebook is no longer canonical.

Contracts belong only to named types. They do not attach to variables, mutable
or immutable bindings, ordinary struct fields, or individual storage locations.

Implementation is partial. Named integer contracts, compile-time literal checks,
ordered membership values, duplicate detection and explicit type defaults are
implemented. The parser retains obsolete variable- and field-contract nodes for
recovery and migration tooling, while Sema rejects them with a stable focused
diagnostic and does not apply them to the storage type. Every `in [...]` member is checked against
the complete named-type contract set.

Static proof of a local contract fact is not by itself a required CTE context.
The contract and default positions listed in "Compile-time-required contract
positions" are explicitly classified as `SemanticCompileTimeRequiredContext`s.
Otherwise canonical runtime contract behavior remains when the value is not
statically established. Runtime validation paths for fallible conversions are
not complete.

## Core rule

A contract restricts the valid semantic values of a named type:

```sec
type Percentage int range 0..100
type Role string in ["admin", "user", "guest"]
type FiniteTemperature float finite
```

Use those types at storage sites:

```sec
let mut percentage: Percentage := 50
let mut role: Role := "user"
let mut temperature: FiniteTemperature := 20.0
```

These obsolete variable-level forms are invalid:

```sec
let mut percentage: int range 0..100 := 50
let mut role: string in ["admin", "user", "guest"] := "user"
let mut temperature: float finite := 20.0
```

Ordinary struct fields use named constrained types:

```sec
type Age int range 0..130

type User struct {
    age: Age,
}
```

Inline field contracts are not Sec 0.1 syntax. They are recognized legacy
syntax and receive a focused migration diagnostic; because a correction would
have to choose a new named type, no automatic rewrite applies
(`rules/foundations/grammar.md`; `rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md` § 4).

## Compile-time-required contract positions

Contract arguments and explicit defaults are ordinary Sec `Expression` syntax;
Sec 0.1 has no separate restricted constant language for them. Every position
that needs its value before runtime lowering is a
`SemanticCompileTimeRequiredContext` under
`rules/compiler/compile_time_evaluation.md`, evaluated by the canonical semantic
CTE evaluator rather than a contract-only evaluator:

```text
range lower bound
range upper bound
each in [...] member
multipleOf divisor
minLen value
maxLen value
exactLen value
regex pattern
explicit default expression
```

The owning rule below supplies each position's required result type and
domain. An immutable value already established by semantic CTE may be read when
ordinary visibility and dependency rules permit it; ordinary mutable static
state is not CTE-readable merely because its initial value was once known.
Ordinary functions, static functions, and instance methods may execute when the
concrete invocation is CTE-legal; no `const fn` category exists. Reading a
property during semantic CTE evaluates its getter, so a static property, or an
instance property whose receiver is available to CTE, may be used when the
executed getter is CTE-legal. Legality is path-sensitive: only the operations
actually executed must be CTE-permitted. Runtime-dependent values remain invalid
because the required evaluation cannot complete:

```sec
let MinimumPort: int := 1
let MaximumPort: int := 0xFFFF

fn DefaultPort() int {
    return 8080
}

type Port int
    range MinimumPort..MaximumPort
    default DefaultPort()
```

Argumentless contracts (`odd`, `even`, `finite`, `notEmpty`, `unique`) introduce
no argument expression but participate fully in compile-time validation of every
compile-time-known value or default (MD-011; `rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md` § 3).

## Composition

Contracts written sequentially are implicit conjunction. Every contract must
pass, in source order:

```sec
type PositiveEven int range 1..100 even
type SmallStep int range 0..100 multipleOf 5
```

Sec does not introduce `and`, `or`, or `not` as contract-composition keywords.
Alternative finite values use `in [...]`; arbitrary validation requires a
separately specified mechanism.

The compiler rejects contract sets that are provably unsatisfiable, including
`odd even`, `odd` combined with an even `multipleOf`, and empty intersections of
known integer ranges and divisibility constraints.

## Applicability

The initial contracts apply as follows:

| Contract | Applicable type families |
|---|---|
| `range` | numeric and compatible unit-bearing named types |
| `in [...]` | named types whose values support compile-time equality |
| `odd`, `even`, `multipleOf` | integer-like named types |
| `finite` | float and decimal named types |
| `regex` | string-like named types |
| `minLen`, `maxLen`, `exactLen`, `notEmpty` | string and supported collection-shaped named types |
| `unique` | supported collection-shaped named types with comparable elements |

The compiler rejects a contract that does not apply to the named base type.

## Range

Canonical shape:

```sec
type Percent int range 0..100
type Port int range 1..<65536
```

Each bound is an ordinary expression evaluated in a
`SemanticCompileTimeRequiredContext` and must produce a value compatible with the
named base type; it is not limited to a signed numeric literal. The range
operator controls inclusivity according to the range grammar. Runtime-dependent
contract bounds are not part of Sec 0.1 because their required evaluation cannot
complete:

```sec
let MaximumPort: int := 0xFFFF

type Port int range 1..MaximumPort
```

The compiler validates constant initializers and explicit defaults at compile
time. A numeric type without an explicit default uses zero when valid, otherwise
the unique exactly representable valid value nearest zero. An equal-distance tie
requires an explicit default. Full default semantics are in `default_values.md`.

## Ordered membership

Canonical shape:

```sec
type Role string in ["admin", "user", "guest"]
type RetryCount int in [1, 3, 5]
```

The list:

- must contain at least one compile-time constant;
- must preserve source order;
- must not contain duplicates;
- must contain only values compatible with the named base type;
- must contain only values satisfying every other contract on the type.

Invalid:

```sec
type EmptyRole string in []
type DuplicateRole string in ["admin", "admin"]
type InvalidEven int in [1, 2, 3] even
```

Invalid membership entries are declaration errors. They are not silently
filtered. Without an explicit type default, the first listed value is the
default.

Membership uses normal Sec semantic equality. It performs no text coercion,
case folding, numeric narrowing, or comparison by memory identity.

## Integer contracts

`odd` accepts integer values with a nonzero low bit. `even` accepts integer
values with a zero low bit. They cannot appear together.

`multipleOf divisor` requires a nonzero integer divisor established by semantic
CTE. Its sign does not affect validity:

```sec
type PageOffset int multipleOf 4096
type PositiveEven int range 1..100 even
```

All integer constraints participate in compile-time consistency and default
resolution; checking each independently is insufficient.

## String and collection contracts

`regex pattern` requires a string pattern established by semantic CTE and
applies to string-like named types.

The canonical production is `RegexContract ::= Contextual("regex") Expression`
under `TypeContract` (`rules/foundations/grammar.md`), superseding the earlier
`ConstantExpression` shape. `regex` is a reserved contract spelling and is
unavailable as a user declaration name (`rules/foundations/lexical_structure.md`
§ 7.3). It has no same-line requirement (MD-009;
`rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md` § 10).

The regular-expression language itself is not defined by this rulebook. Regular
expressions are useful outside type contracts, so Sec requires one dedicated
regex rulebook that owns the pattern syntax, escape interpretation over decoded
Sec strings, Unicode matching semantics, anchoring, whole-value validation
versus search, matching unit, character classes, flags, captures, unsupported
constructs, complexity guarantees, compiled-pattern semantics, invalid-pattern
diagnostics, runtime regex APIs, and contract integration. The `regex` contract
reuses that language and must never define a second, contract-private regex
grammar. Until the dedicated rulebook is accepted, regex contracts cannot be
validated (MD-010; `rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md` § 2).

`minLen`, `maxLen`, and `exactLen` take nonnegative integer values established
by semantic CTE.
`notEmpty` means length greater than zero. String length uses the same unit as
ordinary Sec string-length operations.

`unique` requires every direct element of the contracted collection to be
semantically unequal to every other direct element. It does not recurse into
nested elements. The element type must support equality.

## Finite

`finite` applies to float and decimal named types and excludes NaN and infinity
where the representation supports them:

```sec
type FiniteTemperature float finite
```

A range does not replace `finite`; code must not rely on NaN comparison behavior
as validation. On decimal representations without NaN or infinity the contract
may be representation-trivial while remaining semantically meaningful.

## Explicit defaults

The canonical clause follows every contract:

```sec
type Port int range 1..65535 default 8080
type Role string in ["admin", "user", "guest"] default "user"
```

The default is an ordinary expression evaluated in a
`SemanticCompileTimeRequiredContext`. Its evaluation may use transient
evaluator-local allocation where semantic CTE permits it, but CTE evaluability
and static materializability remain separate: the resulting value must be
representable, materializable under the canonical default-value rules without
hidden runtime allocation, and satisfy every contract, including argumentless
contracts:

```sec
type EvenValue int even default 4    // valid
type EvenValue int even default 3    // compile-time contract error
```

An invalid explicit default invalidates the type declaration; the compiler never
substitutes an implicit default (MD-011; `rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md` §§ 3.6–3.10, 3.35–3.38).

Default precedence and aggregate defaultability are defined by
`default_values.md`.

## Initialization and assignment

A compile-time literal initializer can be proved when the declaration is
analyzed:

```sec
let mut percentage: Percentage := 50
```

Runtime conversion into a constrained named type is fallible. Mutation of a
constrained named value therefore uses the canonical `try` assignment form:

```sec
try percentage = Percentage(value) {
    Err(error) => {
        discard error
    }
}
```

Compound assignment follows the same rule. There is no hidden variable-level
contract setter; the invariant belongs to the named type.

## Conversion failure layers

Conversion into a constrained named type distinguishes three semantic layers,
which must never collapse into one undifferentiated conversion error:

1. **No conversion relation.** When the source and target types have no defined
   explicit conversion, the program is invalid with a compile-time type
   diagnostic. No runtime `Result` error represents an illegal relationship
   between orthogonal types (`int("hello")` when no `string -> int` conversion
   exists).
2. **Intrinsic target-domain failure.** A defined conversion may fail because the
   runtime source value cannot form a valid value of the underlying primitive or
   scalar type: overflow or non-representability, forbidden precision loss, an
   invalid Unicode scalar value, or another validity failure owned by the target
   type. The intrinsic range of `uint8` is a property of the primitive type, not
   an implicit `range` contract.
3. **Declared contract failure.** Contracts are checked only after the value is
   valid for the underlying target domain. A failure here belongs to named-type
   validation.

For a conversion into a constrained named type the order is: verify the
conversion relation exists, evaluate the source, convert into the underlying
target domain and stop on intrinsic failure, then evaluate the declared contracts
in canonical source order and stop on the first failure; only then does the
constrained value exist:

```sec
type SmallOdd int8
    range 1..100
    odd
```

```text
1000    intrinsic int8 representability failure
110     range contract failure
50      odd contract failure
51      success
```

A compile-time-known invalid value is diagnosed at compile time rather than
lowered into a runtime failure. The public runtime error type or types, their
variant names, and payloads remain undecided (MD-012); implementations must
preserve the three layers before that decision (`rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md` § 4).

## Diagnostics

Stable diagnostics are required for:

- inapplicable or unsatisfiable contract sets;
- empty or duplicate membership lists;
- membership values incompatible with the base type;
- membership values violating another contract;
- invalid, unrepresentable, or runtime-dependent explicit defaults;
- ambiguous or unavailable implicit defaults;
- fallible constrained assignment outside `try`.

Diagnostics should point to both the offending value/default and the relevant
contract declaration when practical.

## Related rulebooks

- `default_values.md` defines default selection and defaultability.
- `types.md` defines named identity, declarations, conversions and assignment.
- `operators.md` defines constant-expression operator semantics.
- `struct.md` requires named constrained field types.
- `collections.md` defines collection types and `shaped-types.md` defines shaped types.
- `diagnostics.md` defines diagnostic structure and stability.
