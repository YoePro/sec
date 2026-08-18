# Units

- **Status:** Normative
- **Created:** 2026-08-18
- **Last updated:** 2026-08-18
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/types/units.md`
- **Replaces:** `rules/types/units.txt`
- **Repository baseline reviewed:** `20b3606`

---

## Purpose

Sec units attach semantic meaning to numeric quantities so that the compiler can
reject or warn about numerically valid but semantically suspicious operations.

The unit system is designed to prevent mistakes such as:

- mixing incompatible physical units;
- confusing quantities with the same machine representation;
- confusing frequency with rotational frequency;
- confusing one currency with another;
- losing semantic meaning when deriving rates, ratios, or dimensions;
- silently applying an unknown runtime conversion relationship;
- silently rounding or truncating a unit conversion.

Unit checking is a compile-time semantic feature. A backend representation must
not be used to infer unit meaning.

Implementation status is maintained outside this rulebook.

---

## Core principles

A unit is semantic identity.

A unit is not inherently tied to one numeric representation.

The following concepts are separate:

```text
unit identity
numeric carrier
category
dimension
kind
system
transform
scale and transform metadata
```

For example, the unit `m` may be carried by different numeric types:

```sec
let exactDistance: decimal<m> := 10
let measuredDistance: float64<m> := 10
let counterDistance: int64<m> := 10
```

All three quantities use the same unit identity `m` while using different
numeric carriers.

A unit declaration may provide a default numeric representation. The default is
only shorthand selection. It is not part of the unit's semantic identity.

Unit semantics must survive semantic analysis and Semantic IR until all unit
validation, conversion, diagnostics, and lowering decisions are complete.
Runtime values do not need to carry unit metadata after those decisions have
been fully lowered.

---

## Unit declarations

Canonical declaration syntax:

```sec
unit m physical
```

A declaration may specify a preferred/default numeric representation:

```sec
unit Item uint other
```

Conceptually:

```text
unit Name [DefaultNumericRepresentation] [Category]
```

If the default numeric representation is omitted, it is `decimal`.

If the category is omitted, it is `other`.

Canonical public and standard-library units should normally spell their category
explicitly.

The optional default numeric representation must be a supported plain numeric
scalar carrier. It must not itself be unit-bearing.

Compiler-known hardware layout forms such as `bit` and `register` are not unit
numeric carriers.

The default numeric representation does not restrict explicit use of another
compatible numeric carrier.

Example:

```sec
unit Item uint other

let count: <Item> := 4
let exactCount: decimal<Item> := 4
```

`<Item>` uses `uint` because the unit declares it as its default numeric
representation. `decimal<Item>` uses the same unit identity with a different
numeric carrier.

---

## Unit names and compiler-known names

A unit name must not collide with a compiler-known type, type constructor,
keyword, or other name reserved by the language in the relevant namespace.

In particular, these are not valid user unit declarations:

```sec
unit bit information
unit byte information
```

`bit` has compiler-known hardware semantics and `byte` is a compiler-known type.

Information units should use non-conflicting names or symbols, for example:

```sec
unit octet uint information
unit KiB uint information
```

A unit symbol stored in metadata may still be `"B"`, `"KiB"`, or another
human-facing spelling that does not need to be the source identifier.

---

## Unit categories

Sec 0.1 defines these compiler-known unit categories:

```text
physical
currency
information
ratio
other
```

The category controls high-level semantic and diagnostic policy.

Category is not the same as dimension or system.

### `physical`

`physical` is used for physical quantities that participate in dimensional
analysis.

Examples include:

```text
length
mass
time
force
energy
frequency
rotational frequency
```

A physical unit normally declares an explicit `Dimension`.

### `currency`

`currency` is used for currencies and currency-denominated quantities.

Different named currencies remain semantically distinct even when they use the
same numeric carrier.

Examples:

```sec
unit EUR currency
unit SEK currency
```

If a currency unit omits `Dimension`, the compiler assigns it a unique currency
semantic axis derived from the unit identity. The compiler must not infer a
runtime exchange rate from dimensional compatibility.

### `information`

`information` is used for quantities representing information amount.

Its canonical base dimension is:

```text
information
```

An information unit that omits `Dimension` defaults to:

```text
[information^1]
```

Systems such as IEC belong in `System`, not in the category.

Example:

```sec
unit KiB uint information

impl KiB {
    LongName: "Kibibyte"
    Symbol: "KiB"
    Dimension: [information^1]
    Scale: 8192
    System: IEC
}
```

### `ratio`

`ratio` is used for semantically named dimensionless quantities.

Examples include:

```text
percent
permille
ppm
basis point
```

A ratio unit defaults to an empty dimension vector:

```text
[]
```

A named ratio retains nominal identity even though its mathematical dimension is
dimensionless.

### `other`

`other` is used for application and domain-specific quantities that do not need
an additional compiler-known category policy.

Examples may include:

```text
Item
Packet
Batch
Request
```

`other` does not mean that the compiler ignores the unit. Domain units still
participate in type checking and unit algebra.

If an `other` unit omits `Dimension`, the compiler assigns a unique semantic
base axis derived from the unit identity.

---

## Category provenance in derived expressions

A structural unit expression may combine factors from multiple categories.

Examples:

```text
<EUR/s>
<EUR/SEK>
<KiB/s>
<Packet/s>
```

Such a structural expression is not forced into exactly one source category.
The compiler retains category provenance for semantic checks and diagnostics.

For example, a structural expression containing a currency factor remains known
to contain currency even when divided by a physical time unit.

---

## Dimensions

A dimension is a normalized mapping from semantic axes to non-zero integer
exponents.

Canonical syntax:

```sec
impl m {
    Dimension: [length^1]
}

impl mps {
    Dimension: [length^1, time^-1]
}
```

The old spelling using multiplication inside dimension metadata is not
canonical:

```text
length * 1
```

Canonical Sec 0.1 uses:

```text
length^1
```

Dimension rules:

- every exponent is a non-zero compile-time integer;
- an axis may occur only once in one declared `Dimension` vector;
- declaration order is not semantically significant;
- multiplication adds exponents;
- division subtracts exponents;
- axes whose resulting exponent is zero are removed;
- an empty normalized dimension vector is dimensionless.

The canonical SI base axes are:

```text
length
mass
time
electric_current
thermodynamic_temperature
amount_of_substance
luminous_intensity
```

Additional compiler-known or domain axes may be defined by the unit model.

---

## Dimensionless identity

Sec has one canonical structural dimensionless identity:

```text
<1>
```

Examples:

```text
<m/m>       -> <1>
<s^-1*s>    -> <1>
```

Named dimensionless units, especially ratio units, do not lose their nominal
identity merely because their normalized dimension is `<1>`.

For example, a named percent value remains percent unless a defined operation or
conversion produces an ordinary dimensionless scalar.

---

## Kind

`Dimension` answers:

```text
What mathematical dimension does this quantity have?
```

`Kind` answers:

```text
What semantic quantity does this value represent?
```

Units may declare:

```sec
Kind: energy
```

or:

```sec
Kind: torque
```

Two units may have the same dimension while having different Kinds.

Example:

```text
energy  -> [mass^1, length^2, time^-2]
torque  -> [mass^1, length^2, time^-2]
```

Equal dimension therefore does not prove semantic compatibility.

Another important example is:

```text
Hz
    Dimension: [time^-1]
    Kind: frequency

rpm
    Dimension: [time^-1]
    Kind: rotational_frequency
```

A fixed numeric relationship may exist while the Kinds remain intentionally
different.

### Known and unknown Kind

`Kind` metadata is optional in Sec 0.1 so that existing catalogs and local units
can migrate incrementally.

If omitted, the unit's Kind is `unknown`.

Unknown Kind must not be used as evidence that two different named units are
implicitly semantically compatible.

When both Kinds are known and differ, the compiler must not perform implicit
cross-unit conversion merely because dimensions and scales match.

An explicit validated conversion may cross Kind boundaries.

### Kind inference

The compiler must not invent Kind solely from a dimension vector.

For example:

```text
<N*m>
```

must not automatically become either energy or torque.

A target named unit may provide the expected Kind and validate the structural
expression when the operation is otherwise compatible.

Addition and subtraction preserve Kind when the operands are compatible.
Multiplication and division generally produce an unknown Kind unless a specific
rule can preserve it without ambiguity.

Multiplication or division by a dimensionless ratio may preserve the other
operand's Kind where the operation is semantically valid.

---

## System

`System` is descriptive and conversion metadata for a named unit system.

Examples include:

```text
SI
Imperial
USCustomary
IEC
Currency
Domain
Other
```

System does not determine dimensional compatibility.

Two units from different systems may be dimensionally compatible.
Two units in the same system may represent completely different dimensions.

`System` must not be used as a replacement for `Category`, `Dimension`, or
`Kind`.

---

## Named units

A declared unit has nominal identity.

Example:

```sec
unit mps physical

impl mps {
    LongName: "Meter per second"
    Symbol: "m/s"
    Kind: velocity
    Dimension: [length^1, time^-1]
    Scale: 1
    System: SI
}
```

The type:

```text
<mps>
```

uses the named unit `mps`.

Named identity is not erased merely because another unit or structural unit
expression has the same dimension and scale.

---

## Structural unit expressions

A unit annotation may contain a structural unit expression.

Examples:

```text
<m/s>
<kg*m/s^2>
<KiB/s>
<EUR/s>
<EUR/SEK>
<m^2>
<1>
```

A structural unit expression supports:

- named unit factors;
- multiplication with `*`;
- division with `/`;
- signed integer exponents with `^`;
- parentheses when needed for grouping;
- the structural identity `1`.

The compiler resolves and canonicalizes structural expressions semantically.

For example, factor ordering does not change the structural result:

```text
<kg*m/s^2>
<m*kg/s^2>
```

are structurally equivalent after canonicalization.

The formatter must not replace a structural expression with a named unit or
reorder source factors merely because the compiler canonicalizes them
internally.

---

## Named unit versus structural unit expression

These are intentionally distinct:

```text
<mps>   named unit
<m/s>   structural unit expression
```

They may describe compatible mathematical quantities while retaining different
source-level identity.

If a structural expression is checked against a target named unit, the compiler
may validate the expression against the target's:

- dimension;
- scale/transform relationship;
- known Kind;
- point/difference role;
- numeric carrier exactness.

Without target context, the compiler must not arbitrarily choose one named unit
when several named units could match the same structural result.

An anonymous structural result remains structural unless an unambiguous language
rule or explicit target gives it named identity.

---

## Unit-bearing numeric types

A unit-bearing numeric type consists of:

```text
numeric carrier + unit descriptor
```

Explicit form:

```sec
let distance: decimal<m> := 10
let sampleRate: uint64<Hz> := 48000
```

The numeric carrier controls numeric representation and ordinary numeric
representability.

The unit descriptor controls unit identity and semantic compatibility.

A numeric cast and a unit conversion are different operations.

Changing `uint64` to `decimal` does not by itself change the unit.
Changing `m` to `mm` does not by itself authorize changing the numeric carrier.

---

## Unit-only type shorthand

A unit-only type annotation uses angle brackets:

```sec
let distance: <m> := 10
```

For a single named unit, `<Unit>` selects that unit's declared default numeric
representation.

If no default was explicitly declared, the default is `decimal`.

Therefore:

```sec
unit m physical
```

makes:

```text
<m>
```

shorthand for a quantity equivalent to:

```text
decimal<m>
```

For a compound structural unit expression, the default numeric carrier is
`decimal` unless an explicit numeric carrier is written.

Example:

```text
<m/s>          default decimal carrier
float64<m/s>   explicit binary floating-point carrier
```

This rule avoids accidentally inheriting an unsuitable integer carrier from one
factor of a derived expression.

---

## Unit metadata

Unit metadata is declared in the unit's ordinary `impl` block.

Canonical metadata names are PascalCase.

Sec 0.1 recognizes these unit metadata fields:

```text
LongName
Symbol
BaseUnit
Status
Dimension
Kind
Scale
System
Transform
Offset
Origin
LogBase
LogFactor
Reference
```

The parser may accept older case/underscore variants for migration, but
canonical generated and formatted metadata spelling is PascalCase.

### `LongName`

Human-readable unit name.

### `Symbol`

Human-readable symbol used by documentation, formatting, or tooling.

The symbol does not define source identity.

### `BaseUnit`

Marks a unit as the catalog's canonical/base unit where the unit system uses that
concept.

### `Status`

Allowed values:

```text
active
deprecated
obsolete
```

Default:

```text
active
```

Use of deprecated or obsolete units may produce diagnostics according to the
normal diagnostic policy.

### `Dimension`

The normalized semantic dimension vector.

### `Kind`

Optional semantic quantity kind.

### `Scale`

The linear scale relative to the canonical quantity representation.

`Scale` must be a positive, non-zero compile-time numeric expression.
Exact rational forms are preferred where practical.

### `System`

Named unit system metadata.

### Transform metadata

`Transform`, `Offset`, `Origin`, `LogBase`, `LogFactor`, and `Reference` are
defined below.

---

## Transform model

A unit's transform describes how its numeric coordinate relates to its canonical
quantity representation.

Sec 0.1 defines these transform modes:

```text
linear
affine
logarithmic
```

If `Transform` is omitted, it defaults to:

```text
linear
```

Transform is independent of Category.

A physical unit may be logarithmic.
A ratio unit may be logarithmic.
A physical unit may use an affine transform.

---

## Linear transform

Linear units use `Scale`.

Conceptually:

```text
canonical = value * Scale
```

Example:

```sec
unit mm physical

impl mm {
    LongName: "Millimeter"
    Symbol: "mm"
    Kind: length
    Dimension: [length^1]
    Scale: 1 / 1000
    System: SI
}
```

`Transform: linear` may be written explicitly but is normally omitted.

---

## Affine transform

Affine units use:

```text
Scale
Offset
```

Conceptually:

```text
canonical = value * Scale + Offset
```

An affine unit that represents a point must also define a compatible `Origin`.

Affine conversion must not be reduced to scale-only arithmetic.

The compiler must preserve the difference between point values and differences.

---

## Logarithmic transform

A logarithmic unit uses transform metadata instead of pretending that ordinary
`Scale` alone describes its representation.

Canonical metadata includes:

```text
Transform: logarithmic
LogBase
LogFactor
Reference
```

Conceptually, a logarithmic value maps to a compatible linear quantity through
its declared base, factor, and reference quantity.

`Reference` must resolve to a compile-time reference compatible with the target
linear quantity domain.

For a dimensionless logarithmic ratio, a dimensionless reference such as `1`
may be valid.

Example shape:

```sec
unit dB ratio

impl dB {
    LongName: "Decibel"
    Symbol: "dB"
    Dimension: []
    Kind: ratio
    Transform: logarithmic
    LogBase: 10
    LogFactor: 10
    Reference: 1
    System: Other
}
```

Ordinary linear arithmetic must not be silently applied to logarithmic unit
values.

Unless a specialized rule explicitly permits an operation, arithmetic on
logarithmic values must use explicit conversion to a compatible linear quantity
or an explicitly defined logarithmic operation.

---

## Point and difference semantics

Some quantities represent points in a coordinate space rather than free
vectors/differences.

Examples include:

- absolute temperature readings;
- timestamps;
- altitudes relative to a declared datum;
- other measurements with a meaningful origin.

`Origin` marks the canonical origin used by a point quantity.

A linear transform may still represent a point when `Origin` is present.
Therefore point semantics are not identical to affine transform semantics.

Example concept:

```text
Kelvin
    Transform: linear
    Origin: absolute_zero

Celsius
    Transform: affine
    Origin: absolute_zero
```

`Offset` defines the coordinate displacement; `Origin` defines the semantic
point space.

### Point algebra

For compatible point `P` and difference `D` quantities:

```text
P - P -> D
P + D -> P
P - D -> P
D + D -> D
D - D -> D
```

The following is invalid:

```text
P + P
```

Multiplication and division of point quantities are invalid unless a specialized
rule explicitly defines their meaning.

Comparisons between points require compatible dimension, Kind, origin, and a
valid conversion relationship.

### Difference result

Subtracting two compatible points produces a difference quantity.

For an affine unit, the offset cancels when producing the difference. The
difference uses scale semantics, not point-offset semantics.

Sec 0.1 may infer an anonymous difference quantity.

This rulebook does not introduce a general source syntax such as
`difference<Celsius>`.

When a stable explicit type is required, code may use a separately named linear
difference unit or another compatible declared difference quantity.

A future version may add dedicated ergonomic syntax for point/difference type
relationships without changing the semantic model defined here.

---

## Ratio semantics

Ratios are mathematically dimensionless but semantically significant.

A named ratio unit preserves its name, scale, and Kind.

Example:

```sec
unit percent ratio

impl percent {
    LongName: "Percent"
    Symbol: "%"
    Dimension: []
    Kind: ratio
    Scale: 1 / 100
    System: Other
}
```

When semantically valid, multiplication by a linear ratio acts dimensionally as
multiplication by a scalar while preserving the other operand's dimension and
Kind.

The compiler must not erase a named ratio prematurely merely because its
normalized dimension is empty.

---

## Currency semantics

Currencies are nominal and runtime relationships between different currencies
must never be invented by the compiler.

Example:

```sec
unit EUR currency
unit SEK currency
```

These remain different units.

A value in `SEK` is not implicitly converted to `EUR` through an external or
current market rate.

### Derived currency expressions

Structural currency expressions are valid:

```text
<EUR/s>
<SEK/EUR>
<EUR/SEK>
```

They are not language errors.

Because such expressions can easily be mistaken for compiler-known exchange
relationships, the compiler should emit a warning-class diagnostic by default
when an anonymous derived structural unit contains currency through
multiplication or division.

The warning must not make otherwise valid code fail to compile under the normal
warning policy.

Tooling may distinguish especially suspicious currency ratios from ordinary
currency-per-time rates, but both remain valid unit expressions.

### Runtime/configured currency conversion

Sec supports an explicit factor-provided conversion form:

```sec
let eur := EUR(sek, factor)
```

For a source quantity in `SEK` and a target `EUR`, `factor` must have compatible
unit algebra equivalent to:

```text
<EUR/SEK>
```

The compiler validates:

```text
SEK * EUR/SEK -> EUR
```

The compiler validates the unit relationship but does not validate or invent the
runtime market value of `factor`.

The factor may be runtime or configured data.

This two-argument factor form applies to linear factor-based conversions. It is
not sufficient for affine or logarithmic conversions unless their specialized
rules explicitly reduce to such a factor.

---

## Information semantics

Information quantities use Category `information`.

Example:

```sec
unit KiB uint information

impl KiB {
    LongName: "Kibibyte"
    Symbol: "KiB"
    Kind: information_amount
    Dimension: [information^1]
    Scale: 8192
    System: IEC
}
```

A structural rate is valid:

```text
<KiB/s>
```

The result combines information amount with time dimension.

The names `bit` and `byte` remain unavailable for user unit declarations because
they already have compiler-known language meanings.

---

## Arithmetic

Unit-bearing arithmetic is validated after ordinary numeric operator validity.

The unit system does not make an otherwise invalid numeric operation valid.

### Addition and subtraction

Addition and subtraction require compatible quantity semantics.

The compiler must consider:

- dimension;
- named identity or valid conversion relationship;
- known Kind;
- transform;
- point/difference role;
- numeric carrier exactness.

Compatible linear units of the same known Kind may use an exact fixed conversion
path when the language context permits it.

Different known Kinds are not implicitly combined merely because dimensions
match.

Point arithmetic follows the point rules rather than ordinary scalar addition.

Logarithmic arithmetic follows specialized logarithmic rules and must not fall
through to linear arithmetic.

### Multiplication and division

For ordinary linear quantities, multiplication and division combine structural
unit expressions by adding or subtracting dimension exponents.

The result normally has structural unit identity.

The compiler must not invent a named result or Kind when several semantic
interpretations are possible.

Target context may validate the result against a named target.

Examples:

```text
<m> / <s>       -> <m/s>
<KiB> / <s>     -> <KiB/s>
<Packet> / <s>  -> <Packet/s>
```

Point operands and logarithmic operands are restricted as defined by their
specialized rules.

### Remainder

Remainder requires compatible unit-bearing numeric operands and preserves the
left operand's unit semantics when the underlying numeric remainder operation is
valid.

### Comparison

Equality and ordering require compatible quantity semantics and a valid exact or
explicitly permitted conversion path.

The compiler must not compare different currencies by inventing an exchange
rate.

---

## Exact fixed conversions

A fixed conversion may be compiler-known when all of the following hold:

- source and target dimensions are compatible;
- source and target point/difference roles are compatible;
- known Kinds are compatible;
- the transform relationship is completely known;
- the conversion is representable without hidden loss in the chosen numeric
  carrier;
- the language context permits an implicit fixed conversion.

Example candidates include metric prefix changes such as `m` and `mm`.

A fixed relationship does not imply implicit conversion when Kinds differ.

For example, `Hz` and `rpm` may have a fixed scale relationship but different
Kinds. Their conversion remains explicit.

---

## No hidden precision loss

The compiler must not silently round, truncate, overflow, or otherwise lose
numeric information merely to make a unit conversion fit a target carrier.

If an otherwise valid unit conversion is not exactly representable in the target
numeric carrier, implicit conversion is not permitted.

The programmer must choose one of:

- a more suitable numeric carrier;
- an explicit numeric conversion;
- an explicit rounding policy;
- another explicitly defined conversion operation.

Unit compatibility and numeric representability are both required.

---

## Explicit unit conversion functions

A unit may define explicit conversion functions in its ordinary `impl` block.

A conversion function whose name equals the target unit participates in
constructor-style conversion resolution.

Example:

```sec
unit Hz physical
unit rpm physical

impl Hz {
    Kind: frequency
    Dimension: [time^-1]
    Scale: 1
    System: SI
}

impl rpm {
    Kind: rotational_frequency
    Dimension: [time^-1]
    Scale: 1 / 60
    System: Other

    fn rpm(hz: <Hz>) <rpm> {
        return decimal<rpm>(hz * 60)
    }
}
```

The exact body may use ordinary explicit numeric/unit conversion syntax as
permitted by the type and conversion rules.

Conversion-function rules:

- the conversion is explicit at the call site;
- the function name equals the target unit declaration name;
- the result uses the target unit;
- overloads are resolved by the source quantity type;
- duplicate conversions from the same source type are invalid;
- source and target dimensional relationship must be valid;
- crossing a known Kind boundary is allowed only because the conversion is
  explicit and validated;
- assignment alone must not silently call a user-defined conversion function.

An explicitly called conversion function is authoritative for that call even if
a fixed scale path also exists.

---

## Constructor-style unit conversion

Unit conversion and numeric construction may appear call-shaped.

Examples include:

```sec
let exact: decimal<m> := decimal<m>(value)
let eur := EUR(sek, factor)
```

Sema resolves whether the call is:

- numeric carrier conversion;
- unit conversion;
- explicit unit conversion function;
- factor-provided unit conversion;
- another valid callable form.

The parser must preserve enough information for semantic resolution and must not
infer conversion semantics from spelling alone.

---

## Unit identity and named types

A unit and a named type are related but distinct concepts.

A unit supplies semantic quantity identity.
A named type may add independent nominal identity and contracts.

Example:

```sec
type Distance decimal<m>
type Altitude decimal<m>
```

`Distance` and `Altitude` are different named types even though both use the same
numeric carrier and unit.

Named type identity continues to follow the normal named-type rules.

Unit compatibility does not erase named type identity.

---

## Contracts on unit-bearing named types

A unit-bearing named type may use contracts when the contract is valid for its
numeric carrier and quantity semantics.

Example:

```sec
type PositiveDistance decimal<m> range 0..
```

Contracts belong to the named type, not to the unit declaration itself unless a
separate unit metadata rule explicitly defines a unit invariant.

Unit conversion must not bypass named-type contracts.

---

## Structural result selection

When multiplication or division produces a structural unit result, the compiler
must preserve that structural result unless target context or another explicit
rule identifies a named target.

If several named units share compatible dimension and scale, the compiler must
not pick one arbitrarily.

This is especially important where different Kinds share one dimension.

Example:

```text
<N*m>
```

must not become energy or torque without semantic context.

---

## Evaluation order

Unit checking must not change Sec expression evaluation order.

Conversions that require runtime computation are ordinary evaluated operations
at their source position.

Compile-time unit metadata checks do not create hidden source-visible evaluation.

---

## Diagnostics

Unit diagnostics must distinguish at least:

- unknown unit;
- invalid unit expression;
- incompatible dimension;
- incompatible known Kind;
- incompatible point/difference role;
- incompatible origin;
- invalid affine operation;
- invalid logarithmic operation;
- invalid transform metadata;
- invalid dimension metadata;
- impossible or lossy implicit conversion;
- missing explicit runtime/configured conversion;
- deprecated or obsolete unit use;
- anonymous derived currency warning.

A diagnostic should report both the numeric type and unit semantics when that
information helps explain the error.

Example shape:

```text
cannot implicitly convert frequency Hz to rotational_frequency rpm
help: use an explicit unit conversion
```

Warnings must remain warnings unless project diagnostic policy promotes them.

In particular, `<EUR/s>` and `<SEK/EUR>` are valid expressions and must not be
rejected merely because currency participates in structural algebra.

---

## LSP requirements

The LSP must consume unit facts produced by the compiler rather than implement a
second independent unit algebra.

Hover and related semantic information should expose, when known:

```text
unit name
symbol
numeric carrier
default numeric representation
category
dimension
kind
system
transform
scale
offset
origin
logarithmic metadata
canonical structural form
point or difference role
conversion exactness
```

The LSP should surface unit warnings while editing, including derived currency
warnings and deprecated/obsolete unit use.

For an incompatible operation, tooling should show the semantic reason, not only
the underlying numeric type mismatch.

---

## Formatter requirements

Canonical unit declaration metadata spelling is PascalCase.

Canonical dimension syntax uses `^` integer exponents:

```text
[length^1, time^-1]
```

Canonical structural unit expressions use compact operator spacing:

```text
<kg*m/s^2>
```

The formatter must not:

- replace `<m/s>` with `<mps>`;
- replace a named unit with a structural expression;
- reorder factors merely to match the compiler's internal canonical form;
- change unit identity;
- invent a conversion.

Semantic canonicalization belongs to the compiler, not source rewriting.

---

## Compiler semantic representation

The compiler must preserve sufficient information to distinguish:

- the numeric carrier;
- named versus structural unit identity;
- source named factors;
- normalized structural dimension;
- category provenance;
- known or unknown Kind;
- System;
- transform mode;
- Scale;
- Offset;
- Origin;
- LogBase;
- LogFactor;
- Reference;
- point/difference role;
- conversion plan and exactness.

The compiler must not reduce all unit-bearing quantities to only a dimension
vector.

Doing so would lose distinctions such as energy versus torque and Hz versus rpm.

---

## Semantic IR and lowering

Semantic IR must retain resolved unit semantics until unit-sensitive operations
have been made explicit.

A conversion inserted or required by Sema must be represented explicitly enough
for later verification and lowering.

After all semantic unit behavior has been lowered into explicit numeric
operations, runtime checks, or conversion calls, backend IR may erase unit
metadata that no longer affects runtime behavior.

LLVM or target code must never be asked to rediscover source-language units from
numeric representation.

---

## Standard-library unit catalog

Standard-library units should declare complete metadata appropriate to their
category.

For ordinary linear physical units, the catalog should normally provide:

```text
LongName
Symbol
BaseUnit
Status
Dimension
Kind
Scale
System
```

`Transform: linear` may be omitted because linear is the default.

Existing units that do not yet declare `Kind` remain syntactically valid, but the
catalog should add Kind where it materially improves semantic safety, especially
for units sharing one dimension with different meanings.

Affine and logarithmic units require their additional transform metadata before
they are considered semantically complete.

---

## Compiler-known defaults by category

The compiler may synthesize only these safe defaults:

```text
no explicit numeric representation
    -> decimal

no explicit category
    -> other

ratio with no Dimension
    -> []

information with no Dimension
    -> [information^1]

currency with no Dimension
    -> unique currency axis derived from named unit identity

other with no Dimension
    -> unique semantic axis derived from named unit identity

no Transform
    -> linear

no Status
    -> active
```

The compiler must not synthesize:

- a Kind from dimension alone;
- an exchange rate;
- an affine offset;
- a logarithmic reference;
- an origin;
- a named result from an ambiguous structural expression.

---

## Validation of metadata

The compiler must validate unit metadata before using it for algebra.

At minimum:

- `Scale` is positive and non-zero where required;
- Dimension exponents are non-zero compile-time integers;
- duplicate Dimension axes are rejected;
- `Transform` is one of the compiler-known transform modes;
- affine point units provide required offset/origin information;
- logarithmic units provide valid base, factor, and reference metadata;
- metadata expressions are compile-time values where the field requires that;
- `Reference` is compatible with the logarithmic quantity domain;
- `BaseUnit` declarations do not create contradictory canonical units within one
  catalog domain;
- unit identifiers do not collide with compiler-known reserved names;
- default numeric representations are valid numeric carriers.

Invalid metadata is a compile-time error.

---

## Explicit versus implicit conversion policy

Sec distinguishes four important cases.

### Same named unit

No unit conversion is required. Numeric carrier conversion may still be needed.

### Different named units with fixed exact compatible relationship

An implicit conversion is permitted only when:

- the language context permits it;
- known Kinds are compatible;
- point/difference semantics are compatible;
- conversion is exact in the destination carrier;
- no runtime/configured factor is required.

Otherwise conversion must be explicit.

### Different known Kinds

Conversion is explicit even if dimension and fixed scale relationship are known.

### Runtime/configured relationship

Conversion is explicit and must receive or call the relationship explicitly.

Currency conversion is the primary example:

```sec
let eur := EUR(sek, factor)
```

---

## Interaction with `try`

Unit conversion itself is not automatically fallible merely because it changes
units.

If a conversion operation can fail because of:

- numeric overflow;
- explicit checked rounding policy;
- runtime lookup;
- external conversion service;
- another fallible implementation detail;

that operation must expose ordinary Sec `Result` semantics and use `try` like any
other fallible operation.

The unit system must not hide such failure.

---

## Interaction with registers and FFI

Units may describe the semantic meaning of numeric hardware or foreign values.

A unit annotation does not change ABI representation by itself.

Register field width, FFI ABI type, and unit semantics remain separate.

FFI metadata or wrappers may use units to validate arguments and results, but a
foreign ABI does not gain hidden runtime unit metadata.

Runtime/configured conversion relationships must remain explicit at the Sec
boundary.

---

## Future unit-polymorphic generics

Sec 0.1 does not define generic parameters over units.

A future design may support concepts such as:

```sec
fn Add[U](a: decimal<U>, b: decimal<U>) decimal<U> {
    return a + b
}
```

This example is conceptual future syntax, not Sec 0.1 syntax.

Current generic parameters are type parameters. Unit parameters require a
separate design.

The Sec 0.1 unit model must preserve unit identity independently from numeric
carrier so that future unit polymorphism can be added without redesigning the
basic quantity model.

---

## Non-goals for Sec 0.1

This rulebook does not define:

- arbitrary user-defined unit categories;
- arbitrary user-defined transform modes;
- unit generic parameters;
- automatic market exchange-rate discovery;
- hidden network/configuration lookup for conversion;
- general source syntax for anonymous difference types;
- automatic semantic Kind inference from dimension alone;
- runtime reflection that requires every numeric value to carry unit metadata.

These may be extended by later rulebooks only when they preserve the safety model
defined here.

---

## Summary of invariants

The following rules are normative:

1. A unit is semantic identity and is not inherently bound to one numeric type.
2. A unit-bearing quantity combines a numeric carrier with unit semantics.
3. Named units and structural unit expressions are distinct.
4. Dimension alone does not prove semantic compatibility.
5. Known Kind protects quantities with equal dimensions but different meaning.
6. The compiler does not invent Kind, exchange rates, origins, offsets, or
   logarithmic references.
7. Currency-derived structural expressions are legal and warning-worthy, not
   language errors merely because they contain currency.
8. Runtime/configured conversions are explicit.
9. Point and difference quantities obey affine-space algebra.
10. Logarithmic values do not silently use ordinary linear arithmetic.
11. Named dimensionless ratios retain semantic identity.
12. Unit conversion must never hide rounding, truncation, or other precision
    loss.
13. Compiler and LSP use one shared semantic unit model.
14. Unit metadata may be erased only after all semantic consequences have been
    made explicit in lowering.
