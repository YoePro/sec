# Type Contracts

## Status

This is the canonical Sec rulebook for type contracts. Its canonical filename is
`contracts.md`; the obsolete variable-contract rulebook is no longer canonical.

Contracts belong only to named types. They do not attach to variables, mutable
or immutable bindings, ordinary struct fields, or individual storage locations.

Implementation is partial. Named integer contracts, compile-time literal checks,
ordered membership values, duplicate detection and explicit type defaults are
implemented. The parser and Sema still require an audit to reject every obsolete
variable- and field-contract form. Every `in [...]` member is checked against
the complete named-type contract set. Runtime validation paths for fallible
conversions are not complete.

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

Inline field contracts are not Sec 0.1 syntax.

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

Bounds are compile-time constants compatible with the named base type. The
range operator controls inclusivity according to the range grammar. Runtime-
dependent contract bounds are not part of Sec 0.1.

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

`multipleOf divisor` requires a nonzero compile-time integer divisor. Its sign
does not affect validity:

```sec
type PageOffset int multipleOf 4096
type PositiveEven int range 1..100 even
```

All integer constraints participate in compile-time consistency and default
resolution; checking each independently is insufficient.

## String and collection contracts

`regex pattern` requires a compile-time string pattern and applies to
string-like named types. The concrete regular-expression syntax and engine must
be fixed before runtime validation is implemented.

`minLen`, `maxLen`, and `exactLen` take nonnegative compile-time integer values.
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

The value must be a representable, allocation-free compile-time constant that
satisfies every contract. An invalid explicit default invalidates the type
declaration; the compiler never substitutes an implicit default.

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
- `diagnostics.txt` defines diagnostic structure and stability.
