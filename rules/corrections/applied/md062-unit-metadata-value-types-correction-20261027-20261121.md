# MD-062 — Complete Unit Metadata Public Value Types and Compiler-Known Access

- **Status:** Owner-approved normative correction; implementation and canonical rulebook integration pending.
- **Created:** 2026-10-27
- **Last updated:** 2026-10-27
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Repository baseline:** `YoePro/sec`, `main`, `5426af6cf8cce6fe85ff897b97050dfd330ef4fe` (verified 2026-10-27).
- **Decision:** `MD-062` — unit metadata member value types; design complete.
- **Proposed canonical correction path:** `rules/corrections/pending/md062-unit-metadata-value-types-correction-20261027.md`; move to `rules/corrections/applied/` only after normative synchronization.
- **Normative owners to amend:** `rules/types/units.md`; `rules/foundations/grammar.md`; `rules/types/types.md`; `rules/foundations/operators.md`; `rules/compiler/compiler_known_members.md`; `rules/library/core-library.md`; `rules/compiler/compile_time_evaluation.md` where relevant.
- **Related language contracts:** `rules/declarations/enums.md`; `rules/projects/modules.md`; `rules/compiler/semantic_ir.md`; `rules/tooling/lsp.md`; `rules/tooling/formatter.md`; applicable Sec MLIR/lowering specifications.
- **Implementation tracking:** Reconcile the existing `frontend.units-v2` entry in `governance/types.yaml`, and the existing Semantic IR/MLIR/lowering owners. Preserve stable governance IDs and avoid duplicates.
- **Related decisions:** MD-040 (qualified unit syntax), MD-048 (affine/logarithmic conversion formulas and domain behavior), MD-047 (explicit rounding), MD-066 (ordinary decimal arithmetic result-scale policy).
- **Supersedes:** `MD-062-unit-metadata-value-types-partial-decision-20261027.md` in full, including its superseded `IsBaseunit` public spelling and all formerly deferred MD-062 type choices. **The only approved public spelling is `BaseUnit`.**
- **Change discipline:** Amend canonical paragraphs in place or append stable numbered paragraphs. Never renumber previously cited sections. A correction file is a design decision record, not proof of implementation.

---

## § 1. Authority, scope, and boundaries

**§ 1(1).** This correction completely settles the public Sec value types and read semantics of the 14 existing `impl <Unit>` metadata fields plus the two implicit declaration-derived members `Name` and `Category`: 16 public metadata members in total.

**§ 1(2).** A named unit is referenced as `<U>` (MD-040). Metadata reads use `<U>.Member`, including from ordinary runtime functions. A unit declaration remains `unit Name [DefaultNumericRepresentation] [Category]`, and metadata declarations remain inside `impl <U>`. Nothing in this correction introduces unqualified unit references, unit generics, public type reflection, dynamic metadata mutation, or arbitrary user operator overloading.

**§ 1(3).** Each metadata read is an immutable, typed, compile-time-known value. It can be constant-folded and can be materialized in ordinary executable code. The implementation must not depend on a mutable runtime unit registry, a runtime lookup by unit spelling, or hidden metadata attached to every quantity value.

**§ 1(4).** Public metadata value projection is distinct from compiler-internal exact unit semantics. In particular, exposing `Scale` as `decimal` must not replace exact rational scales used to prove and execute unit conversions.

**§ 1(5).** MD-048 retains authority over exact forward/inverse affine and logarithmic conversion laws, validation of logarithmic domain conditions, and specialized numeric errors. MD-062 specifies the types and observations of their metadata, not those unresolved mathematical conversion plans.

**§ 1(6).** The owner has noted a possible generic `Rational` value type for Sec 0.2. It is not a Sec 0.1 declaration and is not a dependency of this correction.

---

## § 2. Exhaustive public metadata inventory

**§ 2(1).** These 16 members are the complete unit metadata observation surface defined by MD-062. Every name is PascalCase and is the exact public spelling. `Name` and `Category` are implicit; the remaining members correspond to established metadata declarations. No `IsBaseunit`, `IsBaseUnit`, `IsBase`, or second flag alias is introduced.

| `<U>` member | Public value type | Source | Omitted-value behavior |
| --- | --- | --- | --- |
| `Name` | `string` | Unit declaration identifier | Always available for a named unit. |
| `Category` | `UnitCategory` | Unit declaration category | `UnitCategory.Other`. |
| `LongName` | `string` | `LongName:` metadata | Compile-time diagnostic on read if absent. |
| `Symbol` | `string` | `Symbol:` metadata | Compile-time diagnostic on read if absent. |
| `BaseUnit` | `bool` | `BaseUnit:` metadata | `false`. |
| `Status` | `UnitStatus` | `Status:` metadata | `UnitStatus.Active`. |
| `Dimension` | `UnitDimension` | `Dimension:` or category-derived dimension | Apply § 8; error on read if not established. |
| `Kind` | The *concrete enum type* of its value | `Kind:` metadata | `UnitKind.Unknown`. |
| `Scale` | `decimal` | `Scale:` metadata | `1` for linear units; other modes under § 9. |
| `System` | The *concrete enum type* of its value | `System:` metadata | `UnitSystem.Unspecified`. |
| `Transform` | `UnitTransform` | `Transform:` metadata | `UnitTransform.Linear`. |
| `Offset` | `decimal` | `Offset:` metadata | `0` for linear units; affine requirements remain mandatory. |
| `Origin` | The *concrete enum type* of its value | `Origin:` metadata | `UnitOrigin.None`; this does not invent a physical origin. |
| `LogBase` | `decimal` | `LogBase:` metadata | Compile-time diagnostic on read if absent. |
| `LogFactor` | `decimal` | `LogFactor:` metadata | Compile-time diagnostic on read if absent. |
| `Reference` | `decimal` or `decimal<U>` | `Reference:` metadata | Compile-time diagnostic on read if absent. |

**§ 2(2).** Public `Kind`, `System`, and `Origin` have *type-dependent* enum results. An explicitly supplied user enum value preserves its exact nominal enum type; the implicit absent-value defaults have the standard enum types in the table. Their members are not widened to `any`, strings, a general untyped enum, or a universal tagged union.

**§ 2(3).** The declaration's optional default numeric carrier (for example `uint` in `unit Item uint other`) continues to determine the carrier of the `<Item>` shorthand, but is **not** a seventeenth metadata member. Sec 0.1 does not turn types into ordinary first-class values. `Name` is the source declaration identifier, not `LongName`, `Symbol`, or an invented numeric-type name.

**§ 2(4).** All reads require a resolvable **named** unit `<U>`. A structural expression such as `<m/s>` is not assigned a fictional `Name`, `Category`, `Status`, or mutable catalog metadata; derived structural quantity information remains in the existing semantic unit model and tooling contract. No extension of public metadata reads to structural expressions is granted here.

**§ 2(5).** Explicit malformed metadata always produces its applicable diagnostic; the compiler must not quietly use an omission default to conceal a malformed explicit value. Source order, import order, and metadata declaration order do not change the public type or value.

---

## § 3. Canonical public core enums

**§ 3(1).** These are source-visible, nominal, ordinary **closed** Sec enums in `sec/core`; their enum declarations and documentation must be synchronized into the owning core/type rules. They have ordinary Sec enum equality and `match` behavior. The compiler registers exactly their public identities; no separate shadow enum types may be invented in Sema, Semantic IR, the LSP, or stdlib.

**§ 3(2).** `UnitKind` provides a substantial common vocabulary. Its catalog is extensible by *future additions*; an ordinary enum value cannot itself have a dynamically added variant. The exact initial members are:

```sec
enum UnitKind {
    // Kind has not been supplied or otherwise established.
    Unknown

    // A declared miscellaneous quantity without a narrower standard kind.
    Other

    // SI base quantities.
    Length
    Mass
    Time
    ElectricCurrent
    ThermodynamicTemperature
    AmountOfSubstance
    LuminousIntensity

    // General quantities.
    Count
    Ratio
    Probability
    MonetaryAmount
    InformationAmount
    InformationRate

    // Geometry.
    Area
    Volume
    PlaneAngle
    SolidAngle

    // Motion.
    Speed
    Velocity
    Acceleration
    Jerk
    RotationalFrequency
    AngularVelocity
    AngularAcceleration

    // Mechanics.
    Force
    Pressure
    Stress
    MassDensity
    Energy
    Work
    Heat
    Torque
    Power
    ApparentPower
    ReactivePower

    // Time-dependent quantities.
    Frequency
    MassFlowRate
    VolumeFlowRate

    // Electricity and magnetism.
    ElectricCharge
    ElectricPotential
    ElectromotiveForce
    ElectricFieldStrength
    ElectricalResistance
    ElectricalConductance
    ElectricalImpedance
    ElectricalReactance
    Capacitance
    Inductance
    MagneticFieldStrength
    MagneticFlux
    MagneticFluxDensity

    // Thermodynamics.
    Entropy
    HeatCapacity
    SpecificHeatCapacity
    ThermalConductivity

    // Chemistry.
    Concentration
    AmountConcentration
    CatalyticActivity

    // Photometry.
    LuminousFlux
    Illuminance
    Luminance

    // Radiation.
    Radioactivity
    AbsorbedDose
    DoseEquivalent
}
```

**§ 3(3).** `UnitSystem` is descriptive metadata. `Historical` is required; historical units such as region- or date-specific aln, famn, and cubit retain distinct unit identities and their actual declared transformations. A matching system label never proves fixed convertibility or a currency exchange relationship.

```sec
enum UnitSystem {
    // No system was explicitly supplied.
    Unspecified

    // An explicitly miscellaneous system.
    Other

    // International System of Units.
    SI

    // Centimetre-gram-second and its electromagnetic variants.
    CGS
    CGSGaussian
    CGSElectrostatic
    CGSElectromagnetic

    // International Electrotechnical Commission conventions.
    IEC

    // Customary and specialist systems.
    Imperial
    USCustomary
    Nautical
    Astronomical
    Natural
    Planck
    Atomic
    Currency
    Domain

    // Historical systems and regional/period measurements.
    Historical
}
```

**§ 3(4).** `UnitOrigin` exposes basic origins. The names `Zero` and `One` express declared coordinate-origin conventions, not universal proof of a common physical datum. Whether two point quantities share a usable physical origin remains subject to the existing point/origin compatibility model and MD-048. Do not infer that two unrelated historical height datums are compatible simply because both use `Zero`.

```sec
enum UnitOrigin {
    // No physical/coordinate origin was supplied.
    None

    // A declared zero-origin coordinate convention.
    Zero

    // A declared one-origin coordinate convention.
    One

    // Absolute zero for compatible thermodynamic point quantities.
    AbsoluteZero
}
```

**§ 3(5).** `UnitTransform` is closed to the three already established modes. No user-defined transform modes are accepted by MD-062; default is `Linear`.

```sec
enum UnitTransform {
    Linear
    Affine
    Logarithmic
}
```

**§ 3(6).** `UnitCategory` is also closed because categories control compiler semantic/default policy. This enum is the type of implicit `<U>.Category`; it does not introduce a `Category:` key in `impl`.

```sec
enum UnitCategory {
    Physical
    Currency
    Information
    Ratio
    Other
}
```

**§ 3(7).** `UnitStatus` is closed. `Historical` / `Historic` is **not** a status case: historical provenance belongs in `UnitSystem.Historical`, while `UnitStatus` describes catalog lifecycle. A historical unit may be `Active`. The default is `Active`.

```sec
enum UnitStatus {
    Active
    Deprecated
    Obsolete
}
```

**§ 3(8).** Enum comment placement is canonical Sec style: documentation of a variant appears immediately **before** the variant, never after it. Future expansion of standard `UnitKind` or `UnitSystem` must be a deliberate update to the actual core declarations; not a runtime/open-enum loophole.

---

## § 4. User-defined enum metadata without open enums

**§ 4(1).** A `Kind:`, `System:`, or `Origin:` metadata expression may resolve to a member of an ordinary, statically declared Sec enum other than the standard enum for that field. No syntax or semantic change to ordinary enum closedness follows.

```sec
enum LaboratoryKind {
    ParticleFlux
    ReactionRate
}

unit Flux uint other

impl <Flux> {
    Kind: LaboratoryKind.ParticleFlux
}

let selected := <Flux>.Kind
// selected has exact type LaboratoryKind.
```

**§ 4(2).** A typed member is identified by its **nominal enum type identity and member identity**, using canonical module/declaration identity, not merely its spelling. `LaboratoryKind.ParticleFlux` and `OtherKind.ParticleFlux` are distinct, with no implicit cast or compatibility due to their shared variant name.

**§ 4(3).** The expression in the metadata field must resolve unambiguously to a compile-time-known member of one concrete enum type. Runtime enum variables, strings, arbitrary integers, `any`, reflection, or a dynamic enum-type selector are not valid metadata declarations.

**§ 4(4).** `Kind`, `System`, and `Origin` observations preserve their concrete enum type. When absent they are the explicitly typed standard defaults `UnitKind.Unknown`, `UnitSystem.Unspecified`, and `UnitOrigin.None`. Their defaults do **not** establish a positive semantic compatibility fact between otherwise unrelated units.

**§ 4(5).** Neither matching dimensions nor matching variant spellings may erase category provenance, point/difference role, named unit identity, or nominal enum type identity. Standard `UnitKind.Energy` and a user enum member named `Energy` are not interchangeable.

**§ 4(6).** `Transform`, `Category`, and `Status` do not follow this extensibility rule. Their values must be members of `UnitTransform`, `UnitCategory`, and `UnitStatus`, respectively, with the declared fixed member sets.

---

## § 5. Existing declaration syntax and migration

**§ 5(1).** Canonical *newly authored* enum-backed metadata uses a qualified enum member. Examples:

```sec
unit m physical

impl <m> {
    LongName: "Metre"
    Symbol: "m"
    BaseUnit: true
    Status: UnitStatus.Active
    Dimension: [length^1]
    Kind: UnitKind.Length
    Scale: 1
    System: UnitSystem.SI
    Transform: UnitTransform.Linear
}

unit dB ratio
impl <dB> {
    Dimension: []
    Kind: UnitKind.Ratio
    Transform: UnitTransform.Logarithmic
    LogBase: 10
    LogFactor: 10
    Reference: 1
    System: UnitSystem.Other
}
```

**§ 5(2).** Existing Sec 0.1 catalogs currently contain legacy bare metadata names such as `Kind: energy`, `Kind: temperature`, `Transform: logarithmic`, `System: SI`, `Origin: absolute_zero`, and `Status: active`. They must remain accepted during the 0.1 migration period and normalize to the corresponding typed standard enum cases. In particular, historical `temperature` maps to `UnitKind.ThermodynamicTemperature`, not to a newly invented extra `Temperature` case.

**§ 5(3).** The integration must enumerate the actual legacy vocabulary from `sec/core`, `sec/stdlib/unit`, and existing conformance fixtures, record all supported exact aliases in one canonical migration mapping, and test every entry. A formerly allowed arbitrary bare token that has no unambiguous standard mapping must receive a migration diagnostic requiring `CustomEnum.Member`; the compiler shall not manufacture an anonymous standard variant or guess a user enum by member-name search. Existing valid catalog declarations must not be broken by incompletely enumerated migration aliases.

**§ 5(4).** Migration handling is contextual to unit metadata. It does not add global language keywords, implicit enum conversions or case-insensitive enum member lookup in ordinary Sec expressions. The canonical formatter may rewrite legacy metadata names only under a documented migration policy that preserves resolved meaning; no unrelated source token rewriting.

**§ 5(5).** Existing `BaseUnit:` is the canonical field spelling and `<U>.BaseUnit` its public member; there is no `IsBaseunit` alias. The older partial MD-062 document is superseded and MUST NOT override this owner's later explicit correction.

**§ 5(6).** Duplicate definitions of one metadata key on the same named unit are invalid, including case/underscore migration spellings normalized to the same key and duplicates spread across multiple `impl <U>` blocks. Diagnostics identify both declarations. A malformed explicit declaration is not equivalent to absence.

---

## § 6. Implicit `Name` and `Category`

**§ 6(1).** `Name: string` and `Category: UnitCategory` are **read-only, compiler-derived** members of every named unit. They are never metadata declarations written by the programmer inside `impl <U>`.

```sec
unit Item uint other

let name := <Item>.Name         // string: "Item"
let category := <Item>.Category // UnitCategory.Other
```

**§ 6(2).** `Name` returns the unit's **declaration identifier** (the defining short source name), not its `LongName`, display symbol, resolved import alias, or implementation-specific unique identifier. The source-visible name must not change based on where a unit is imported.

**§ 6(3).** `Category` returns the resolved category from the unit declaration. When the category is omitted, it returns `UnitCategory.Other` under the established default. A `Category:` metadata line inside `impl` is rejected as an attempt to redefine declaration authority.

**§ 6(4).** `Name:` inside a unit `impl` is similarly forbidden as metadata. Ordinary unit functions are still called with `()` as `<U>.Name(...)` where such functions are otherwise legal; this callable expression must not be confused with the compiler-known **value** member `<U>.Name`. If the implementation cannot distinguish the syntactic forms, it must repair resolution rather than silently shadow a built-in property.

**§ 6(5).** The `unit Name [DefaultNumericRepresentation] [Category]` source grammar remains unchanged; `UnitCategory` is the type of the observed value, not a replacement for the existing declaration category tokens.

---

## § 7. `UnitDimension`: complete Sec 0.1 public contract

**§ 7(1).** `UnitDimension` is a public, nominal, compiler-known, immutable, copyable value type provided as part of `sec/core`. It is a compiler-provided/opaque semantic type, not an ordinary user-writable `struct` exposing a map or an invented generic `Dimension[N]`. Its representation is not public source syntax.

**§ 7(2).** Its complete **specialized** Sec 0.1 public member/operation set is:

| Member / operation | Exact type signature | Meaning |
| --- | --- | --- |
| `AxisCount` | Read-only property `AxisCount: uint` | Number of non-zero axes in the normalized dimension. |
| `IsDimensionless` | Read-only property `IsDimensionless: bool` | True exactly when the normalized axis set is empty. |
| `==` | `(UnitDimension, UnitDimension) -> bool` | Exact mathematical normalized-dimension equality. |
| `!=` | `(UnitDimension, UnitDimension) -> bool` | Logical negation of equality. |

**§ 7(3).** Neither `AxisCount` nor `IsDimensionless` has a setter or a distinct `Get...` method. The compiler-known property reads are `dimension.AxisCount` and `dimension.IsDimensionless`; the equality operations are ordinary Sec source operators resolved specially for this compiler-known nominal type, not user operator overloads.

**§ 7(4).** There are **no** public constructors, writable fields, literal constructors, index operators, iterators, dimension multiplication/division operators, implicit conversions to strings/arrays/maps or other specialized public operations for `UnitDimension` in 0.1. The general universal type rules continue to apply where applicable; this correction does not create a special `ToString` overload or a public reflection API.

**§ 7(5).** `UnitDimension` is **not defaultable**; an actual, compiler-resolved dimension is required to initialize a value. It is freely copyable, has no destructor or owned resource, can be stored in aggregates and passed/returned by value, and needs no heap allocation for ordinary metadata reads or equality. It has no guaranteed foreign/FFI ABI representation.

**§ 7(6).** A dimension is canonically a finite mapping from **semantic axis identity** to non-zero **mathematical integer exponent**. Exponents are normalized by addition on multiplication and subtraction on division; zero-result entries are erased. Member order, original spelling order and internal map iteration order are semantically irrelevant. The empty mapping is dimensionless.

**§ 7(7).** `a == b` is true **iff** the two canonical mappings have identical axis identities and identical exponents for every axis; `a != b` is its logical negation. Equality is independent of unit `Name`, `Category`, `Kind`, `Scale`, `System`, `Transform`, point/difference role and storage identity. Two units of different Kinds, including energy and torque, may have equal dimensions **without being semantically interchangeable**.

**§ 7(8).** The canonical SI base axes `length`, `mass`, `time`, `electric_current`, `thermodynamic_temperature`, `amount_of_substance`, and `luminous_intensity` have fixed shared compiler-known identities. The established `information` axis has its fixed shared identity. Additional compiler-known axes may be defined by their unit-model owner.

**§ 7(9).** A user/domain axis has an identity including its **defining `ModuleIdentity`** and the declared axis name, not just the unqualified string. Two independent modules using identical unqualified axis spellings create distinct axes. Imports and qualified axis references must preserve the defining origin identity; when the same exported axis is legally referenced through different import aliases, it remains the same axis. This requirement must be met consistently by Sema, module surfaces, incremental facts and serialization. Merely matching the printed name never proves equality.

**§ 7(10).** Sec 0.1 does not introduce a new standalone axis declaration syntax. Existing `Dimension: [axis^exponent, ...]` syntax is retained, including qualified axes already admitted by the grammar. The implementation must resolve imported qualified axes against actual accessible unit/domain metadata and reject unresolved axes; it may not invent cross-module axis equality based on coincidence of spelling or synthesize arbitrary imported declarations.

**§ 7(11).** Every declared exponent is a non-zero compile-time integer; duplicate resolved axis identities are rejected even if source spellings differ by aliasing. A normalized result may cancel to dimensionless. Compiler numeric width must not silently overflow exponents; excessive exponents are rejected with deterministic diagnostics or resource-limit failures, never wrapped into a different valid dimension.

**§ 7(12).** Metadata reads produce already canonical `UnitDimension` values. The compiler may precompute comparisons but **runtime equality must remain correct** when operands are function parameters or aggregate fields. It may lower using statically materialized immutable descriptors, canonical collision-free IDs or equivalent typed values. A raw hash equality without collision verification and a pointer-identity comparison without guaranteed global interning are both invalid. No mutable global registry or per-quantity runtime unit tag is required.

**§ 7(13).** Cross-module and separate-compilation equivalence is semantic. Two references to one canonical dimension compare equal regardless of object placement, compile order or import alias; different module-local axis identities compare unequal. Compiler facts and materialization use the selected `CompilationPlan` consistently.

**§ 7(14).** Examples (the named units must be declared/imported with the indicated dimensions):

```sec
let length := <m>.Dimension
let sameLength := length == <km>.Dimension       // true
let speed := <mps>.Dimension
let count := speed.AxisCount                   // 2
let empty := <dB>.Dimension.IsDimensionless    // true

fn SameDimension(left: UnitDimension, right: UnitDimension) bool {
    return left == right
}
```

A named-only metadata access on a structural unit expression (for example `<m/s>.Dimension`) is not part of this public API and belongs in negative conformance tests, not the positive examples. The compiler still knows the structural expression's dimension for its normal unit algebra.

---

## § 8. Missing `Dimension` and category defaults

**§ 8(1).** The existing unit category rules determine dimension establishment if `Dimension:` is omitted:

| Declaration category | Established dimension if omitted |
| --- | --- |
| `physical` | **Unknown**; `<U>.Dimension` is a compile-time error until an explicit dimension is established. |
| `currency` | A unique currency axis derived from the named currency unit identity. |
| `information` | `[information^1]`. |
| `ratio` | `[]` (dimensionless). |
| `other` | A unique semantic axis derived from the named unit identity. |

**§ 8(2).** Unknown physical dimension is not equal to an empty dimension and is **not** a `UnitDimension` value. It is an absence-of-knowledge state in compiler semantic facts. A public read when unresolved produces a clear source-positioned diagnostic; it does not return `Option[UnitDimension]`, `UnitDimension.Unknown` or a silently dimensionless result.

**§ 8(3).** A valid declared `Dimension: []` yields a normal, usable dimensionless value with `AxisCount == 0` and `IsDimensionless == true`. A missing `Dimension:` on a `ratio` unit is equivalent under the existing category default. Named unit identity survives even though its dimension is empty.

**§ 8(4).** Existing dimension declaration syntax remains `[axis^signed_integer, ...]`, with a non-zero exponent per declared entry and no duplicated resolved axis. Structural multiplication/division normalization remains a compiler semantic operation; no public mutable map or `UnitDimension` arithmetic is added.

---

## § 9. Scale, Offset, LogBase, LogFactor, Reference

**§ 9(1).** `<U>.Scale` has public type `decimal`, is dimensionless and is a compile-time constant. Existing declarations such as `Scale: 1 / 60` remain valid. The compiler retains their **exact rational values** for unit algebra, conversion proofs and lowering; public decimal materialization is a distinct observation. A decimal approximation does not retroactively change the exact declared unit semantics.

**§ 9(2).** For **linear units**, `Scale` must be **non-zero**, and both positive and negative values are valid. This expressly supersedes the older strictly-positive linear Scale requirement. A negative linear scale reverses coordinate direction; a zero scale is invalid because it is not invertible. Arithmetic/range and conversion proofs must handle sign changes correctly, including swapping interval endpoints where required. This correction does **not** decide additional affine-negative-scale domain details, which remain with MD-048.

**§ 9(3).** An omitted `Scale` of an ordinary linear unit has the exact default `1`. `Offset` of an ordinary linear unit reads as decimal `0` when omitted. For an affine point declaration, the existing requirement to provide a valid `Scale`, `Offset` and `Origin` is mandatory; no omission default may fabricate a valid affine transform. Reading a non-applicable missing `Scale`/`Offset` of a logarithmic unit must not silently fabricate linear-transform semantics.

**§ 9(4).** `Offset`, `LogBase`, and `LogFactor` are public `decimal` values. `LogBase` and `LogFactor` have no invented omission defaults; valid logarithmic metadata remains subject to the existing base/factor checks. Their public decimal projection is separate from exact internal conversion mathematics.

**§ 9(5).** `<U>.Reference` is `decimal` for a dimensionless reference and `decimal<V>` for a reference quantity carrying the named/structural linear unit `V`. Its **concrete** result type is statically resolved from the declared reference; neither a string nor `any` is permitted. The reference itself is compile-time-known. Example shapes:

```sec
Reference: 1
Reference: 0.001<W>
Reference: 1<V>
```

**§ 9(6).** There is **no universal `Reference > 0` rule**. A particular logarithmic conversion must satisfy its actual transformation's domain (including the sign of the logarithm argument). Zero is invalid when it serves as a divisor in a logarithmic ratio; admissible negative reference values are governed by MD-048, not summarily forbidden or declared valid for every transformation by MD-062. `Origin` is not a substitute for `Reference`.

**§ 9(7).** Exposing an exact rational `Scale` as `decimal` may produce a **decimal approximation**, as explicitly accepted for Sec 0.1. This metadata-read projection is defined to select the **nearest representable value of `decimal`**, using `RoundingMode.NearestEven` to break exact halfway ties. The result is a single deterministic projection of the exact rational value into the selected public scalar type; it is not a sequence of intermediate binary-float or decimal roundings. Existing decimal representable-range/scale constraints apply. This is a narrow, documented `Scale`-observation policy, **not** an implicit rounding permission for ordinary unit conversion, `Offset`, `LogBase`, `LogFactor`, `Reference`, or general decimal arithmetic. Precision must not be chosen from the host language, target backend, compiler iteration order or optimization flags. No approximate readback may feed the compiler's exact unit semantics. MD-066 still owns ordinary decimal *operator* result-scale policy. A separate `Rational` type remains a candidate for 0.2.

**§ 9(8).** The compiler must distinguish declaration validity, exact internal rational semantics, and the public decimal readback. If any accepted metadata value cannot be materially represented by the target public type under its governing numeric contract, fail the **read/materialization** with a clear, source-positioned diagnostic rather than altering the underlying unit declaration or emitting an arbitrary value. For `Reference`, MD-048 may add further transformation-specific validation.

---

## § 10. Defaults, invalid values, and access diagnostics

**§ 10(1).** All defaults in § 2 are **typed, ordinary immutable constants**. Missing metadata with an approved default reads as that value; it is not an optional, unknown-typed, runtime-fetched, or deferred symbolic placeholder. `UnitKind.Unknown`, `UnitSystem.Unspecified` and `UnitOrigin.None` remain **absence markers**, not evidence of compatibility.

**§ 10(2).** `Name` and `Category` are available for every named unit. Absent `LongName` and `Symbol` cause compile-time diagnostics **on read**, rather than returning the empty string or secretly synthesizing a display name. Ordinary unit declarations without these optional display fields remain valid.

**§ 10(3).** Absent `BaseUnit` yields `false`. Absent `Status` yields `UnitStatus.Active`. Absent `Kind` yields `UnitKind.Unknown`. Absent `System` yields `UnitSystem.Unspecified`. Absent `Transform` yields `UnitTransform.Linear`. Absent `Origin` yields `UnitOrigin.None` only as observable metadata: it never licenses a point whose origin is required.

**§ 10(4).** `LogBase`, `LogFactor`, and `Reference` have no default. Reading an absent value is a compile-time error. Independently, declaring `Transform: UnitTransform.Logarithmic` without the required three metadata fields remains a declaration-validation error. Neither error hides the other, and the diagnostic must explain the missing property clearly.

**§ 10(5).** When a field is required by a particular declared transform or category, the compiler validates that requirement *even if source never reads the member*. A public read cannot make invalid metadata valid by defaulting it. Mis-typed, incompatible, duplicate, unknown and non-constant expressions remain diagnostics.

**§ 10(6).** Metadata reads are immutable. No assignment, address-taking for mutation or property setter is permitted. Ordinary compiler-known universal members do not permit redefinition of reserved metadata member meaning.

---

## § 11. Semantic typing, names, and evaluation order

**§ 11(1).** The parser retains existing qualified unit reference and metadata grammar. A metadata declaration is parsed only inside an eligible `impl <U>`. `Name` and `Category` remain implicit and are not added to the list of authorable unit metadata keys.

**§ 11(2).** Member resolution first resolves a named unit and the exact member name. If it is one of the 16 metadata value members, it produces the field's typed constant. A parenthesized callable form resolves a legal unit function according to the ordinary call rules; e.g. `<U>.Name` is the implicit `string`, while `<U>.Name(...)` remains a callable if such a function is actually declared and valid. An unavailable function remains an error, not a fabricated metadata call.

**§ 11(3).** `Kind`, `System`, `Origin`, `Transform`, `Status`, and `Category` are ordinary nominal enum-typed values. Compare them with ordinary compatible enum operands; the compiler must not implicitly compare different nominal enum types or narrow to a standard fallback.

**§ 11(4).** Every explicit metadata value is semantically analyzed, validated and bound to canonical declaration identities at compilation. A public read reuses the resolved fact; it must not reparse raw metadata token strings using runtime-context-dependent interpretation.

**§ 11(5).** CTE and runtime uses yield the same semantic values. Compile-time known does not require every surrounding expression to be CTE-only. A normal runtime function may accept `UnitDimension`, strings, enum values, decimal values or `decimal<V>` references without runtime re-resolution of the original unit declaration.

**§ 11(6).** The compiler registers authoritative unit members through the existing typed compiler-known-member registry with stable IDs; aliases, when explicitly permitted for migration, do not become distinct metadata state. The `UnitDimension` equality operator uses the existing operator resolution authority, not a user-defined operator declaration.

---

## § 12. Compiler, core, IR, MLIR, LSP and formatter synchronization

**§ 12(1).** Add the six enum declarations from § 3 to the canonical `sec/core` public surface with documentation comments before affected declarations/members. Register those types in ordinary enum/nominal type lookup, `match`, equality and constant evaluation. Prevent duplicate parallel synthetic symbols from being created for the same core declarations.

**§ 12(2).** Provide the `UnitDimension` public nominal opaque identity with the **exact** two read-only members and two equality operators from § 7. Document the complete compiler-provided public signature/structural contract in the owning rulebook and generated LSP documentation. Do not publish a fictitious user-writable source struct or unimplemented constructor bodies merely to illustrate its private representation.

**§ 12(3).** Replace the current Sema rejection of `<U>.Kind`, `<U>.Dimension`, and other non-text metadata with exact member resolution. Preserve the existing `<U>.Symbol` and `<U>.LongName` success cases while correcting absent-value behavior. Add `Name`/`Category`, enforce exact `BaseUnit` spelling, and detect duplicate and forbidden metadata declarations.

**§ 12(4).** Replace string-only metadata identity in semantic decisions with canonical nominal enum type/member facts where applicable. `UnitKind`, `UnitOrigin`, `UnitSystem`, `UnitTransform`, `UnitCategory`, and `UnitStatus` remain distinct types. Maintain catalog migration aliases at the boundary; do not erase the enum identity after parsing.

**§ 12(5).** Preserve exact scale and other conversion coefficients (currently represented by `big.Rat` in the Go frontend) separately from typed public decimal values. Negative linear scales must be respected in conversion ratios, value intervals and proof conditions; prior `factor > 0` shortcuts must not silently reject accepted negative conversions or incorrectly order interval extrema.

**§ 12(6).** Enrich dimension axis facts with canonical module-aware identity; equality, normalization, hash/fingerprint, cache validity, module surfaces and serialization must compare resolved identities rather than naked spellings. Do not replace `Kind` compatibility with dimension-only equivalence.

**§ 12(7).** CTE produces typed values of the canonical public type. Semantic IR retains enough information to distinguish the resolved member, unit declaration, concrete enum identity, normalized dimension and exact-versus-projected decimal fact. Lowering may fold or materialize readonly constants, but must not erase semantics before legality, precision and identity checks are discharged.

**§ 12(8).** Sec MLIR and target lowering must support `UnitDimension` immutable values and both equality predicates when they escape CTE, using a semantically correct finite static representation or equivalent plan-compatible identity. No public data layout/FFI ABI is promised. Metadata lookups and runtime equality must not silently add heap allocation or mutable global registry dependencies.

**§ 12(9).** The LSP consumes Sema-authoritative facts for hover, completions, diagnostics, definitions and signature help. Hover shows the concrete enum result type for custom metadata, not a generic `Enum` or `any`; `UnitDimension` exposes exactly `AxisCount` and `IsDimensionless` as specialized properties. Missing metadata diagnostics explain how to declare or correct the value.

**§ 12(10).** The formatter preserves `unit` declaration categories, names, existing dimensional syntax and exact written numeric expressions such as `Scale: 1 / 60`. It recognizes typed `UnitKind.Energy` and legacy metadata names in the unit metadata context, remains idempotent, and never changes the resolved unit value merely to match internal normalization. It must not rename `BaseUnit` or emit `IsBaseunit`.

**§ 12(11).** Unit metadata processing must remain valid when a unit and an ordinary type share a spelling, and when the same unit is imported under a module alias. All existing MD-040 qualification rules continue to apply.

---

## § 13. Required positive conformance tests

**§ 13(1).** Test **every one** of the 16 public members for type inference, constant folding, direct read in an ordinary function, and static materialization where applicable. Test declared values and every omission default from § 2. Verify `Name` is the declared short identifier and `Category` is declaration-derived.

**§ 13(2).** Test the full standard enum member inventories, distinct nominal types, and exhaustive matches for `UnitTransform`, `UnitCategory`, and `UnitStatus`. Include `UnitSystem.Historical` and historical units whose status remains `UnitStatus.Active`. Do not collapse `Unknown` into `Other`, or `Unspecified` into `Other`.

**§ 13(3).** Test user-defined `Kind`, `System`, and `Origin` enums in the declaring module and across imports. Assert that member reads have the exact user enum type and can be compared/matched according to that type; same member spellings in different enums remain distinct.

**§ 13(4).** Test `Scale: 1 / 60`, `Scale: 1 / 3`, ordinary decimal scales, and `Scale: -1` on linear units. Conversion analysis must use exact rational values, while the public `decimal` read has the deterministic Sec decimal projection. Test zero-scale rejection and interval reversal for negative scales.

**§ 13(5).** Test dimensionless `Reference: 1`, unitful `Reference: 0.001<W>`, positive and negative typed reference *representation* cases. MD-048 retains responsibility for deciding whether a given logarithmic transform's domain admits particular signs. Assert unit identity is not erased from unitful reference result types.

**§ 13(6).** Test `UnitDimension` equality of `[length^1, time^-1]` and `[time^-1, length^1]`, inequality against another dimension, equality of energy/torque dimensions without inferring compatible Kinds, the empty dimension, cancellation to empty, `AxisCount`, `IsDimensionless` and parameter-based runtime equality.

**§ 13(7).** Test module-axis disambiguation: two modules using the same private axis spelling compare unequal; legally imported references to one originating axis compare equal even through different aliases. Include separate compilation, incremental reuse and cross-module runtime equality.

**§ 13(8).** Test legacy metadata spellings against every actual core/stdlib catalog value and verify their canonical enum mapping. Verify formatter idempotence, LSP hover, Semantic IR facts, MLIR verification and multiple backend/static materialization paths.

---

## § 14. Required negative conformance tests

**§ 14(1).** Reject missing `LongName`, missing `Symbol`, unresolved physical `Dimension`, missing `LogBase`/`LogFactor`/`Reference` public reads, and incomplete affine/logarithmic declarations; verify precise diagnostics distinguish missing data from invalid declarations.

**§ 14(2).** Reject `Name:` or `Category:` inside `impl <U>`; assignment to any metadata member; duplicate keys including case/underscore aliases; malformed typed enum members; out-of-scope numeric/type-reflection values; arbitrary bare identifiers without approved migration mapping; and attempts to use `IsBaseunit` or `IsBaseUnit` as public metadata.

**§ 14(3).** Reject custom enums for `Transform`, `Category` or `Status`; reject cross-enum equality without valid same-type operands; reject interpreting `UnitOrigin.None` as sufficient affine origin; reject treating equal dimensions as sufficient proof of compatible Kinds or currency conversion.

**§ 14(4).** Reject `Scale: 0` for linear units, exponent zero/duplicate axis entries, invalid or overflowed exponent arithmetic, and source metadata expressions that depend on runtime values. Check integer interval proof correctness under negative scale factors.

**§ 14(5).** Reject access to named-only metadata on structural expressions such as `<m/s>.Name` and `<m/s>.Dimension`; reject reading `<U>.DefaultNumericType` or another unapproved seventeenth metadata member. Structural dimension semantics for quantity algebra remain legal internally and are not disabled by this public-read restriction.

**§ 14(6).** Reject `UnitDimension` default construction, user constructors, assignments to its properties, unauthorized indexing/iteration, ordered comparison and arithmetic. Assert there is no hash-only equality collision, pointer-only inequality for equivalent dimensions, runtime registry lookup, accidental allocation or loss of module-origin identity.

**§ 14(7).** Where public `decimal` materialization cannot follow the governing representation contract, produce a deterministic compile-time diagnostic. The exact internal unit conversion must remain unaffected; no silent use of a runtime approximation as its scale.

---

## § 15. Governance and integration acceptance

**§ 15(1).** Integrate into the current normative **owners**, not only this correction. Preserve stable paragraph references, correct the former undecided-value wording in `units.md`, synchronize unit grammar and typed member registry, and add complete public core declarations/documentation. If an older correction remains historically retained, mark its supersession rather than editing its history invisibly.

**§ 15(2).** Reconcile implementation work by the existing governance ID `frontend.units-v2` in `governance/types.yaml`. Add or update machine-readable implementation/remains/identified-bugs records according to the repository's **current** schema without creating duplicate IDs, parallel ad-hoc integration entries or stale legacy lists. Map dependent Semantic IR/MLIR work to its actual existing owner rather than fabricating an untracked package number.

**§ 15(3).** Implementers (Claude/Codex or human) shall proceed in verifiable increments: identify the applicable existing governance item; change one coherent parser/core/Sema/IR/tooling behavior; add its positive/negative test; run relevant tests; then advance. Do not mark the full implementation complete just because this decision has been approved.

**§ 15(4).** Minimum completion checks: unit parser/CST/formatter suites, full Sema unit + enum suites, CTE and compiler-known registry tests, semantic IR/MLIR verifier/lowering tests, LSP tests, module/incremental tests, existing core/stdlib unit catalog tests, `go test ./... -count=1`, race-targeted affected-package tests where practical, `go vet ./...`, YAML validation and `git diff --check`. Any unsupported backend must fail closed with a precise unsupported-lowering diagnostic, not silently discard metadata.

**§ 15(5).** Mark MD-062 **design-resolved** when this normative correction is integrated in its actual rulebooks and the decision registry is synchronized. Implementation may remain `partial` with explicit governance tasks. Remove/close MD-062 in `missing-decisions.yaml` only according to repository policy after normative owner synchronization; do not close MD-048, MD-066 or unrelated work automatically.

**§ 15(6).** This one correction includes the complete Sec 0.1 metadata-type contract. New design-decision IDs are not required for the approved member names, known defaults, standard enums, `UnitDimension` equality/runtime semantics, module-scoped axes or supported linear negative `Scale`. Genuine later extensions (e.g. new logarithmic laws, a general `Rational` in 0.2, additional catalog enum members) remain under their respective future scopes.

---

## § 16. Summary of immutable invariants

**§ 16(1).** All 16 `<U>.Member` accesses have the exact result type from § 2 or a specified compile-time diagnostic. Metadata is read-only, compiler-known and can be used in ordinary code without an implicit runtime unit lookup.

**§ 16(2).** `BaseUnit` is the exact canonical public name; `Name` and `Category` are declaration-derived. `UnitTransform`, `UnitCategory`, `UnitStatus` are closed, and `Kind`, `System`, `Origin` preserve the concrete selected enum's nominal type.

**§ 16(3).** `UnitDimension` equality compares canonical mathematical axes/exponents with defining-module identity for domain axes. It does not imply equality of `Kind` or of unit identity, and does not add general reflection or operator overloading.

**§ 16(4).** Linear `Scale` may be negative but never zero. Compiler unit mathematics uses exact rationals; the public Sec 0.1 read exposes `decimal`, potentially approximate, while future `Rational` research belongs to 0.2.

**§ 16(5).** Missing metadata observes only defined defaults; mandatory transform metadata must still validate. MD-048 continues to govern specialized affine/logarithmic conversion behavior, independently of this completed MD-062 public metadata design.
