# Units

- **Status:** Normative
- **Created:** 2026-08-18
- **Last updated:** 2026-10-27 (MD-062 unit metadata value types)
- **Document revision:** 2.2
- **Sec language version:** 0.1
- **Canonical path:** `rules/types/units.md`
- **Replaces:** `rules/types/units.txt`
- **Repository baseline reviewed:** `20b3606`
- **Implementation governance:** `governance/types.yaml`

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

Static unit conversion/dimension proof is not automatically a semantic-CTE
request. An owning unit/type source position that requires evaluation before
runtime lowering must declare the appropriate required context explicitly. CTE
preserves the exact nominal unit and carrier semantics rather than evaluating
only the underlying representation.

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

Declared unit symbols occupy a separate unit-symbol namespace, not the shared
declaration namespace of `rules/foundations/names_scopes_visibility.md`. A unit
symbol therefore does not conflict with an ordinary identifier of the same
spelling: after `unit s physical`, a local, parameter, or field named `s` is
valid, and unit symbol `<s>` still names the unit. Shadowing, duplicate
declaration, and visually confusable checks do not compare unit symbols with
ordinary identifiers (`rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md` § 8).

The same holds for ordinary type declarations: a project may declare
`type A int` or `type K struct { ... }` although trusted core declares the
units `A` (ampere) and `K` (kelvin), and a unit and an ordinary type of the
same spelling may coexist even within one module (MD-040, 2026-10-13,
`rules/corrections/applied/md040-md046-md047-md056-units-register-bits-rounding-correction-20261013.md`
§ 2).

### Qualified unit references

A reference to a unit **always** uses the `<...>` spelling, whether or not an
ordinary type of the same name exists. A unit declaration keeps the
`unit Name ...` syntax; it is a declaration, not a reference.

```sec
unit Volt physical

type Volt uint

impl <Volt> {
    LongName: "Volt"
    Symbol: "V"
    Kind: UnitKind.ElectricPotential
    Dimension: [mass^1, length^2, time^-3, electric_current^-1]
    Scale: 1
    System: UnitSystem.SI
}

impl Volt {
    fn IsZero() bool {
        return self == 0
    }
}
```

- `impl <Volt>` targets only the unit and holds its metadata and unit
  functions; it must be in the module that declares the unit. `impl Volt`
  targets only the ordinary type. Resolution never depends on declaration
  order, fallback lookup or a preference for types over units. A unit
  impl cannot declare `free`: units express measurement semantics, not a
  destructible instance (MD-025.4, 2026-10-15).
- A unit conversion is called `<Volt>(value)`; a unit function is called
  `<Volt>.Name(...)`; unit metadata is read `<Volt>.Symbol`. The ordinary type
  forms `Volt(value)` and `Volt.SizeOf` keep their meaning, and neither form
  changes the other's namespace.
- Unit annotations keep their forms: `uint16<Volt>`, `decimal<Volt>`,
  `<Volt>` and `5<s>`.
- A bare unit name is never a unit reference: `impl rpm`, `rpm(hz)` and a type
  written `rpm` are errors whose help names the `<rpm>` spelling.

Every unit metadata member, plus the implicit `Name` and `Category`, is
readable as `<U>.Member` with the exact value type given in "Unit metadata
value types" below (MD-062, 2026-10-27).

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

impl <KiB> {
    LongName: "Kibibyte"
    Symbol: "KiB"
    Dimension: [information^1]
    Scale: 8192
    System: UnitSystem.IEC
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
impl <m> {
    Dimension: [length^1]
}

impl <mps> {
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
Kind: UnitKind.Energy
```

or:

```sec
Kind: UnitKind.Torque
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
    Kind: UnitKind.Frequency

rpm
    Dimension: [time^-1]
    Kind: UnitKind.RotationalFrequency
```

A fixed numeric relationship may exist while the Kinds remain intentionally
different.

### Known and unknown Kind

`Kind` metadata is optional in Sec 0.1 so that existing catalogs and local units
can migrate incrementally.

If omitted, the unit's Kind is `UnitKind.Unknown`. `Kind` may also name a
member of a user-declared enum ("Unit metadata value types" (4)); `Unknown` is
an absence marker, never a compatibility fact.

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

The standard systems are the members of the core enum `UnitSystem`
(`rules/library/core-library.md` § 27): `Unspecified`, `Other`, `SI`, `CGS`,
`CGSGaussian`, `CGSElectrostatic`, `CGSElectromagnetic`, `IEC`, `Imperial`,
`USCustomary`, `Nautical`, `Astronomical`, `Natural`, `Planck`, `Atomic`,
`Currency`, `Domain` and `Historical`. `System` may also name a member of a
user-declared enum. Historical units (for example region- or period-specific
aln, famn and cubit) keep distinct unit identities and their declared
transforms; their provenance is `UnitSystem.Historical`, not a status.

A matching system never proves fixed convertibility or a currency exchange
relationship. System does not determine dimensional compatibility.

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

impl <mps> {
    LongName: "Meter per second"
    Symbol: "m/s"
    Kind: UnitKind.Velocity
    Dimension: [length^1, time^-1]
    Scale: 1
    System: UnitSystem.SI
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

## Quantity literals

Decided by MD-015 (2026-10-13,
`rules/corrections/applied/md015-time-unit-duration-conversion-correction-20261013.md`).

A numeric literal immediately followed by a unit annotation is one
unit-bearing numeric quantity:

```text
QuantityLiteral
    ::= (IntegerLiteral | FloatingLiteral) UnitAnnotation

UnitAnnotation
    ::= "<" UnitExpression ">"
```

```sec
let seconds := 5<s>
let milliseconds := 250<ms>
let negative := -5<s>
let fractional := 0.5<ns>
let distance := 100<m>
let velocity := distance / seconds
```

`UnitAnnotation` and `UnitExpression` are the ordinary unit grammar and name
resolution of "Structural unit expressions"; there is one general production,
not a time-only parser. A leading sign is an ordinary unary expression, not
part of the literal.

The `<` must follow the literal without intervening whitespace and the
complete annotation must close on the same line; otherwise `<` is the ordinary
comparison operator (`5 < s`, `5<count`). Formatting keeps the adjacent
spelling (`500<ms>`).

The numeric carrier is the expected unit-bearing numeric type when it has the
same unit coordinates (`let t: int64<ms> := 250<ms>`), otherwise the unit's
default carrier exactly as for the unit-only type shorthand (`5<s>` is
`decimal<s>` unless `s` declares another default). An integral literal does
not by itself select an integer carrier. The literal must initialize that
carrier exactly. Unknown units and malformed unit expressions receive the
ordinary unit diagnostics.

A quantity with the dimension of time stays a unit-bearing quantity; it
becomes a `duration` only through the conversion of
`rules/types/temporal.md` § 5. Only the literal itself is materialized in an
expected-`duration` position; once bound, the quantity keeps its unit type
and needs a written `duration(...)` (MD-015, 2026-10-15). The time units `s`, `ms`, `us` and `ns` are
core units (`rules/types/temporal.md` § 5.2).

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

`Name` and `Category` are not metadata fields: they are implicit,
declaration-derived members, and writing `Name:` or `Category:` inside
`impl <U>` is an error ("Unit metadata value types" (6)). A metadata key
defined twice for one unit, including through migration spellings that
normalize to the same key or across several `impl <U>` blocks, is an error
naming both declarations.

### `LongName`

Human-readable unit name.

### `Symbol`

Human-readable symbol used by documentation, formatting, or tooling.

The symbol does not define source identity.

### `BaseUnit`

Marks a unit as the catalog's canonical/base unit where the unit system uses that
concept. `BaseUnit` is the only spelling; there is no `IsBaseunit`,
`IsBaseUnit` or `IsBase` alias.

### `Status`

Allowed values are the members of the closed core enum `UnitStatus`:

```text
UnitStatus.Active
UnitStatus.Deprecated
UnitStatus.Obsolete
```

Default:

```text
UnitStatus.Active
```

Historical provenance is not a status: a historical unit may be `Active`.

Use of deprecated or obsolete units may produce diagnostics according to the
normal diagnostic policy.

### `Dimension`

The normalized semantic dimension vector.

### `Kind`

Optional semantic quantity kind.

### `Scale`

The linear scale relative to the canonical quantity representation.

For a linear unit, `Scale` must be a non-zero compile-time numeric
expression; positive and negative values are valid (MD-062, superseding the
former strictly-positive rule). A negative scale reverses coordinate
direction: conversion ratios, value intervals and range proofs must handle the
sign, including swapping interval endpoints. A zero scale is invalid because
it is not invertible. Negative scales on affine units remain under MD-048.
Exact rational forms are preferred where practical; the compiler keeps the
exact rational for unit algebra.

### `System`

Named unit system metadata.

### Transform metadata

`Transform`, `Offset`, `Origin`, `LogBase`, `LogFactor`, and `Reference` are
defined below.

---

## Unit metadata value types

This section integrates MD-062
(`rules/corrections/md062-unit-metadata-value-types-correction-20261027.md`,
§§ 1–16). Paragraph numbers are stable.

**(1) Observation surface.** A named unit `<U>` exposes exactly 16 read-only
metadata members: the 14 metadata fields above plus the implicit `Name` and
`Category`. Each read is an immutable, typed, compile-time-known value that
may be constant-folded or materialized in ordinary runtime code. Reads never
depend on a mutable runtime unit registry, a runtime lookup by unit spelling,
or metadata attached to quantity values. Public projection is distinct from
the compiler's exact unit semantics; in particular the `decimal` projection of
`Scale` never replaces the exact rational used for conversions.

| `<U>` member | Value type | Source | When omitted |
| --- | --- | --- | --- |
| `Name` | `string` | declaration identifier | always available |
| `Category` | `UnitCategory` | declaration category | `UnitCategory.Other` |
| `LongName` | `string` | `LongName:` | compile-time error on read |
| `Symbol` | `string` | `Symbol:` | compile-time error on read |
| `BaseUnit` | `bool` | `BaseUnit:` | `false` |
| `Status` | `UnitStatus` | `Status:` | `UnitStatus.Active` |
| `Dimension` | `UnitDimension` | `Dimension:` or category default | (8); error on read if not established |
| `Kind` | concrete enum type of its value | `Kind:` | `UnitKind.Unknown` |
| `Scale` | `decimal` | `Scale:` | `1` for linear units; (9) |
| `System` | concrete enum type of its value | `System:` | `UnitSystem.Unspecified` |
| `Transform` | `UnitTransform` | `Transform:` | `UnitTransform.Linear` |
| `Offset` | `decimal` | `Offset:` | `0` for linear units; affine requires it |
| `Origin` | concrete enum type of its value | `Origin:` | `UnitOrigin.None` |
| `LogBase` | `decimal` | `LogBase:` | compile-time error on read |
| `LogFactor` | `decimal` | `LogFactor:` | compile-time error on read |
| `Reference` | `decimal` or `decimal<V>` | `Reference:` | compile-time error on read |

**(2) Named units only.** Reads require a resolvable named unit. A structural
expression such as `<m/s>` has no `Name`, `Category` or other public
metadata: `<m/s>.Name` and `<m/s>.Dimension` are errors, although the compiler
still knows its dimension for unit algebra. The optional default numeric
carrier (`uint` in `unit Item uint other`) is not a member; `<U>.DefaultNumericType`
or any other seventeenth member is an error. Source, import and declaration
order never change a member's type or value.

**(3) Core enums.** `UnitKind`, `UnitSystem`, `UnitOrigin`, `UnitTransform`,
`UnitCategory` and `UnitStatus` are ordinary nominal closed Sec enums declared
in `sec/core` (`rules/library/core-library.md` § 27), with ordinary enum
equality and `match`. No shadow enum types exist in Sema, Semantic IR, the LSP
or the standard library. `Unknown`, `Unspecified` and `None` are absence
markers: they never establish compatibility between units, and they are not
collapsed into `Other`. `UnitOrigin.Zero` and `UnitOrigin.One` are declared
coordinate conventions, not proof of a shared physical datum; origin
compatibility stays with point semantics and MD-048. Extending `UnitKind` or
`UnitSystem` is a deliberate change to the core declaration, never an open
enum.

**(4) User enum values.** `Kind:`, `System:` and `Origin:` may name a
compile-time-known member of any ordinary declared enum. The read keeps that
enum's exact nominal type, identified by canonical module and declaration
identity, not by spelling: `LaboratoryKind.ParticleFlux` and
`OtherKind.ParticleFlux` are distinct, and a user member named `Energy` is not
`UnitKind.Energy`. Runtime enum variables, strings, integers, `any` and
reflection are not valid metadata values. `Transform:`, `Category` and
`Status:` accept only their core enums.

```sec
enum LaboratoryKind {
    ParticleFlux
    ReactionRate
}

unit Flux uint other

impl <Flux> {
    Kind: LaboratoryKind.ParticleFlux
}

let selected := <Flux>.Kind // LaboratoryKind
```

**(5) Canonical spelling and migration.** Newly written enum-backed metadata
uses qualified members such as `Kind: UnitKind.Length`,
`System: UnitSystem.SI` and `Transform: UnitTransform.Logarithmic`. Legacy
bare names in Sec 0.1 catalogs (`Kind: energy`, `Kind: temperature`,
`Transform: logarithmic`, `System: SI`, `Origin: absolute_zero`,
`Status: active`) remain accepted during the 0.1 migration and normalize to
the standard enum members; `temperature` maps to
`UnitKind.ThermodynamicTemperature`. The implementation keeps one canonical
migration table built from the actual `sec/core`, `sec/stdlib/unit` and
fixture vocabulary, with a test per entry. A bare token with no unambiguous
mapping is a migration diagnostic asking for `CustomEnum.Member`; the compiler
never invents a standard member or searches user enums by member name. The
migration is contextual to unit metadata and adds no keywords, implicit enum
conversions or case-insensitive enum lookup elsewhere. The formatter rewrites
legacy names only under a documented policy that preserves the resolved
meaning.

**(6) `Name` and `Category`.** `Name` is the declaration identifier (not
`LongName`, `Symbol`, an import alias or an internal ID) and does not change
with the importing module. `Category` is the declared category, or
`UnitCategory.Other` when omitted; the `unit` declaration grammar and its
category tokens are unchanged. `<U>.Name` is the value member;
`<U>.Name(...)` still calls a unit function of that name if one is legally
declared, and resolution must keep the two forms apart rather than shadow
either.

**(7) `UnitDimension`.** `UnitDimension` is a public, nominal, compiler-known,
opaque, immutable, copyable `sec/core` value type. Its complete Sec 0.1 public
surface is the read-only properties `AxisCount: uint` (number of non-zero
axes) and `IsDimensionless: bool` (the normalized axis set is empty), and the
operators `==` and `!=` on two `UnitDimension` operands
(`rules/foundations/operators.md`). There are no constructors, literals,
writable fields, setters, `Get...` methods, indexing, iteration, ordering,
arithmetic or implicit conversions, and no special `ToString`. It is not
defaultable, has no destructor or owned resource, may be stored in aggregates
and passed or returned by value, needs no heap allocation, and has no
guaranteed FFI layout.

A dimension is a finite map from semantic axis identity to non-zero
mathematical integer exponent; multiplication adds and division subtracts
exponents and zero entries are erased, so spelling and iteration order are
irrelevant. `a == b` holds iff both maps have the same axis identities with the
same exponents; it is independent of unit name, category, Kind, scale, system,
transform, point/difference role and storage. Equal dimensions (energy and
torque) never make units interchangeable.

The SI base axes (`length`, `mass`, `time`, `electric_current`,
`thermodynamic_temperature`, `amount_of_substance`, `luminous_intensity`) and
`information` have fixed compiler-known identities. A user or domain axis is
identified by its defining `ModuleIdentity` and declared name: two modules
using the same spelling create distinct axes, while one exported axis reached
through different import aliases stays one axis. Sema, module surfaces,
incremental facts and serialization compare resolved identities. No new axis
declaration syntax exists; `Dimension: [axis^exponent, ...]` keeps its form,
qualified axes must resolve against accessible metadata, each exponent is a
non-zero compile-time integer, duplicate resolved axes are rejected, and
exponent overflow is a deterministic diagnostic, never wrapping.

Runtime equality must stay correct for parameters and aggregate fields. It may
use statically materialized descriptors or collision-free canonical IDs;
hash-only equality without collision checking and pointer identity without
guaranteed global interning are invalid. Equality across modules and separate
compilation is semantic and uses the selected `CompilationPlan`.

```sec
let length := <m>.Dimension
let sameLength := length == <km>.Dimension    // true
let count := <mps>.Dimension.AxisCount        // 2
let empty := <dB>.Dimension.IsDimensionless   // true

fn SameDimension(left: UnitDimension, right: UnitDimension) bool {
    return left == right
}
```

**(8) Omitted `Dimension`.** The category defaults in "Compiler-known
defaults by category" apply. For a `physical` unit with no `Dimension:` the
dimension is unknown: that is an absence of knowledge, not a `UnitDimension`
value, not dimensionless, and not an `Option`; reading `<U>.Dimension` is a
source-positioned error. A declared `Dimension: []` (or an omitted one on a
`ratio` unit) is an ordinary dimensionless value with `AxisCount == 0`.

**(9) Numeric members.** `Scale`, `Offset`, `LogBase` and `LogFactor` read as
`decimal`. `Reference` reads as `decimal` for a dimensionless reference and as
`decimal<V>` for a reference quantity in linear unit `V` (`Reference: 1`,
`Reference: 0.001<W>`); its concrete type is resolved statically and its unit
identity is kept. Omitted `Scale` of a linear unit is exactly `1` and omitted
`Offset` of a linear unit is `0`; an affine point unit must still declare
`Scale`, `Offset` and `Origin`, and reading a logarithmic unit's missing
`Scale` or `Offset` never fabricates linear semantics. There is no universal
`Reference > 0` rule: the admissible domain, including zero as a divisor and
negative references, belongs to MD-048.

The `decimal` read of `Scale` projects the exact rational to the nearest
representable `decimal`, breaking exact ties with `RoundingMode.NearestEven`.
It is one deterministic projection, independent of host, target, iteration
order and optimization flags, and is not a rounding permission for unit
conversion, the other numeric members or decimal arithmetic (decimal operator
result scale and division rounding are defined by MD-066 in
`rules/foundations/operators.md` "Decimal arithmetic"; the nearest
representable value ranges over the whole signed `int32` scale domain). The
approximate value never feeds back into exact unit
semantics. When an accepted value cannot be represented in its public type
under the governing numeric contract, the read or materialization fails with a
source-positioned diagnostic; the declaration is not altered.

**(10) Defaults and diagnostics.** Defaults are typed immutable constants, not
optional, deferred or runtime-fetched values. Missing `LongName` or `Symbol`
is an error only on read; declarations without them stay valid. `LogBase`,
`LogFactor` and `Reference` have no defaults, and a logarithmic declaration
missing any of them is a declaration error even when nothing reads it; the
two errors are reported independently. `UnitOrigin.None` never satisfies a
required origin. Explicit malformed, mistyped, duplicate, unknown or
non-constant metadata is always diagnosed and never replaced by a default.
Metadata reads cannot be assigned, mutably addressed or set.

**(11) Resolution.** The member name is resolved after the named unit; one of
the 16 names yields the field's typed constant, reused from the facts bound
during analysis rather than reparsed. Enum-typed members compare only with the
same nominal enum type. Compile-time and runtime uses give the same values,
and runtime functions may take `UnitDimension`, enum, `decimal` and
`decimal<V>` values without re-resolving the unit. The members are registered
through the compiler-known member registry
(`rules/compiler/compiler_known_members.md`, "Unit metadata members"), and
`UnitDimension` equality through ordinary operator resolution, not a user
operator. Unit metadata stays correct when a unit shares its spelling with an
ordinary type and when it is imported under a module alias.

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

impl <mm> {
    LongName: "Millimeter"
    Symbol: "mm"
    Kind: UnitKind.Length
    Dimension: [length^1]
    Scale: 1 / 1000
    System: UnitSystem.SI
}
```

`Transform: UnitTransform.Linear` may be written explicitly but is normally
omitted.

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
Transform: UnitTransform.Logarithmic
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

impl <dB> {
    LongName: "Decibel"
    Symbol: "dB"
    Dimension: []
    Kind: UnitKind.Ratio
    Transform: UnitTransform.Logarithmic
    LogBase: 10
    LogFactor: 10
    Reference: 1
    System: UnitSystem.Other
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
    Transform: UnitTransform.Linear
    Origin: UnitOrigin.AbsoluteZero

Celsius
    Transform: UnitTransform.Affine
    Origin: UnitOrigin.AbsoluteZero
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

impl <percent> {
    LongName: "Percent"
    Symbol: "%"
    Dimension: []
    Kind: UnitKind.Ratio
    Scale: 1 / 100
    System: UnitSystem.Other
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
let eur := <EUR>(sek, factor)
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

impl <KiB> {
    LongName: "Kibibyte"
    Symbol: "KiB"
    Kind: UnitKind.InformationAmount
    Dimension: [information^1]
    Scale: 8192
    System: UnitSystem.IEC
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
- an explicit rounding policy (see "Explicit rounding");
- another explicitly defined conversion operation.

Unit compatibility and numeric representability are both required.

---

## Explicit rounding

Decided by MD-047 (2026-10-13,
`rules/corrections/applied/md040-md046-md047-md056-units-register-bits-rounding-correction-20261013.md`
§ 5).

No unit or numeric conversion silently rounds, truncates, overflows or loses
precision. A program may request one explicit rounding mode as an additional
argument of the ordinary typed numeric or unit conversion; there is no new
keyword or rounding operator:

```sec
let speed: decimal<rpm> := 100
let hz := try uint16<Hz>(speed, RoundingMode.Ceil)
let low := int8(value, RoundingMode.Floor)
```

The argument is a case of the one core `RoundingMode` enum
(`rules/library/core-library.md` § 10.3). Rounding cannot manufacture unit
compatibility: the conversion must still satisfy every type, dimension, Kind,
range and named-type rule, and units with different Kinds still need a
declared conversion.

Rounding happens exactly once, after the exact mathematical unit transform
(scale and, where defined, offset), to a value representable in the target
carrier. No intermediate value is rounded and then transformed again.
Logarithmic and affine details remain with MD-048.

| Mode | 1.6 | -1.6 | Halfway 2.5 | Halfway -2.5 |
|---|---:|---:|---:|---:|
| `NearestEven` | 2 | -2 | 2 | -2 |
| `NearestAwayFromZero` | 2 | -2 | 3 | -3 |
| `TowardZero` | 1 | -1 | 2 | -2 |
| `AwayFromZero` | 2 | -2 | 3 | -3 |
| `Ceil` | 2 | -1 | 3 | -2 |
| `Floor` | 1 | -2 | 2 | -3 |

Without a mode, an exact representable conversion succeeds; a statically known
inexact or out-of-range conversion is a compile-time error, and a dynamic
checked conversion uses `ConversionError`. With a mode, the chosen loss of the
fractional part is permitted, but no other error is: a known out-of-range
result is a compile-time error and a dynamic one fails with
`ConversionError.OutOfRange`. `try` is required exactly when the conversion
remains fallible — not merely because a rounding mode is written; a proven
constant or bounded integer source needs none. Failures use the core
`ConversionError` (`PrecisionLoss`, `OutOfRange`); there is no `RoundingError`.
`duration(q, mode)` follows the same model (`rules/types/temporal.md` § 5.6).

---

## Explicit unit conversion functions

A unit may define explicit conversion functions in its `impl <Unit>` block.

A conversion function whose name equals the target unit participates in
constructor-style conversion resolution and is called `<Unit>(value)`
(MD-040). Other unit functions are called `<Unit>.Name(...)`.

Example:

```sec
unit Hz physical
unit rpm physical

impl <Hz> {
    Kind: UnitKind.Frequency
    Dimension: [time^-1]
    Scale: 1
    System: UnitSystem.SI
}

impl <rpm> {
    Kind: UnitKind.RotationalFrequency
    Dimension: [time^-1]
    Scale: 1 / 60
    System: UnitSystem.Other

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
let eur := <EUR>(sek, factor)
```

A unit conversion or factor-provided conversion is always written with the
qualified unit reference `<Unit>(...)` (MD-040); `decimal<m>(value)` remains a
carrier-qualified conversion.

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

Hover on a metadata read shows the concrete result type from "Unit metadata
value types", including a user enum type for custom `Kind`, `System` or
`Origin`, never a generic enum or `any`. Completion on a `UnitDimension`
offers exactly `AxisCount` and `IsDimensionless`. A missing-metadata
diagnostic explains how to declare or correct the value.

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
- invent a conversion;
- rewrite exact written numeric metadata such as `Scale: 1 / 60`;
- rename `BaseUnit` or emit `IsBaseunit`.

The formatter recognizes typed metadata values such as `UnitKind.Energy` and
legacy bare names in unit metadata, stays idempotent, and rewrites a legacy
name only under the documented migration policy ("Unit metadata value types"
(5)).

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

For metadata reads, Semantic IR keeps the resolved member, the unit
declaration, the concrete enum identity, the normalized dimension and whether
a numeric value is exact or the projected `decimal` read. Lowering may fold or
materialize read-only constants only after legality, precision and identity
checks are discharged. Sec MLIR and target lowering support `UnitDimension`
values and both equality predicates when they escape compile-time evaluation,
with a finite static representation and no heap allocation or mutable global
registry. A path that cannot lower them fails with
`[unsupported-lowering]`, never by discarding metadata.

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
    -> UnitTransform.Linear

no Status
    -> UnitStatus.Active

no Kind
    -> UnitKind.Unknown

no System
    -> UnitSystem.Unspecified

no Origin
    -> UnitOrigin.None (observable only; never satisfies a required origin)

no BaseUnit
    -> false

no Scale on a linear unit
    -> 1

no Offset on a linear unit
    -> 0
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

- `Scale` is non-zero for linear units (negative is valid; MD-062);
- enum-backed metadata names a member of the field's permitted enum (core
  `UnitTransform`, `UnitStatus`; any declared enum for `Kind`, `System`,
  `Origin`), or a legacy name in the migration table;
- no metadata key is defined twice and `Name:`/`Category:` are not declared;
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
let eur := <EUR>(sek, factor)
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

Decided by MD-046 (2026-10-13,
`rules/corrections/applied/md040-md046-md047-md056-units-register-bits-rounding-correction-20261013.md`
§ 4); `rules/declarations/registers.md` § 12 owns the field rules:

- A field `bit[N]<U>` occupies N register bits and carries the unit `<U>`.
  Reading it gives the smallest unsigned carrier of N bits with the unit
  preserved (`bit[12]<rpm>` reads as `uint16<rpm>`) and the field range
  `0..2^N-1`.
- A write proves both the unit identity and the N-bit range. It is never
  truncated and never becomes fallible; an unknown or wider value is narrowed
  explicitly first.
- `bit[N](value)` never inherits a unit. The optional suffix
  `bit[N](value)<U>` gives the result the unit `U`: it labels a unitless raw
  value (the programmer's assertion at the hardware boundary), or keeps a
  unit-bearing source's own unit. It never changes a unit — `bit[12](speed)<Hz>`
  with `speed` in rpm is an error; `<Hz>(speed)` is the conversion. No scale,
  offset, logarithm or rounding is applied.

```sec
let raw: uint16 := 0x1ABC
let frequency: uint16<rpm> := ReadSpeed()

let plain := bit[12](raw)               // uint16; unitless
let speed := bit[12](raw)<rpm>          // uint16<rpm>
let bare := bit[12](frequency)          // uint16; unit explicitly discarded
let retained := bit[12](frequency)<rpm> // uint16<rpm>
let invalid := bit[12](frequency)<Hz>   // error: unit change without conversion
```

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
