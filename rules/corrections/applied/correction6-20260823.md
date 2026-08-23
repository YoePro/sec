# Correction 6 — Sema invents undeclared physical unit semantics

## Audit context

- Repository: `github.com/YoePro/sec`
- Repository baseline: `c515862`
- Audited: `2026-08-19`
- Primary rulebook: `rules/types/units.md`
- Document revision: **2.0**
- Document created: `2026-08-18`
- Document last updated: `2026-08-18`
- Document baseline reviewed: `20b3606`

## Classification

Implementation bug.

This is not a language-rule change.

Revision 2.0 makes unit identity declaration/catalog driven and restricts compiler-synthesized unit defaults to an explicit safe list. Current Sema still carries an older implicit-unit model that:

1. predefines ordinary physical unit names inside the compiler; and
2. synthesizes an identity-derived dimension for arbitrary physical units that omit `Dimension`.

Both behaviors can give source code unit semantics that were never declared by the program or unit catalog.

---

# Normative requirements

`rules/types/units.md` revision 2.0 establishes:

- a unit is semantic identity;
- unit declarations define named unit identities;
- unknown units are diagnostic errors;
- ordinary standard-library units belong to the standard-library unit catalog;
- compiler-known names reserve language concepts such as `bit`, `byte`, intrinsic types, constructors, and unit categories, not an implicit catalog of ordinary SI unit identifiers;
- a `physical` unit normally declares an explicit `Dimension`;
- the compiler may synthesize **only** the listed safe defaults.

The exhaustive safe dimension defaults are:

```text
ratio with no Dimension
    -> []

information with no Dimension
    -> [information^1]

currency with no Dimension
    -> unique currency axis derived from unit identity

other with no Dimension
    -> unique semantic axis derived from unit identity
```

There is no compiler-synthesized `physical` Dimension in that list.

Therefore absence of `Dimension` on a physical unit must not silently become a made-up axis such as `[Temperature^1]`.

---

# Bug 1 — ordinary physical units are compiler-predeclared without source declarations

## Affected code

`internal/sema/units.go`

```go
func builtinUnits() map[string]UnitDefinition {
    units := map[string]UnitDefinition{}

    addPhysical := func(names []string, dimension Dimension) {
        for _, name := range names {
            units[name] = UnitDefinition{
                Name: name,
                Category: PhysicalUnit,
                Dimension: dimension,
                DefaultNumeric: "decimal",
                Status: StatusActive,
                Transform: LinearUnitTransform,
            }
        }
    }

    addPhysical([]string{"m", "metre", "meter"}, ...)
    addPhysical([]string{"mm", "millimetre", "millimeter"}, ...)
    addPhysical([]string{"inch"}, ...)
    addPhysical([]string{"s", "second"}, ...)
    addPhysical([]string{"kg", "kilogram"}, ...)
    addPhysical([]string{"Hz", "Hertz", "hertz"}, ...)
    ...
}
```

`NewAnalyzerWithDepth()` installs this table before any source declarations are processed:

```go
return &Analyzer{
    ...
    units: builtinUnits(),
}
```

## Consequence

Source can currently resolve ordinary unit factors without declaring or importing the corresponding unit identity.

Existing tests demonstrate this legacy behavior:

```sec
type Meter decimal<m>
type Second decimal<s>
type Speed decimal<m/s>
```

with no preceding `unit m` or `unit s` declaration.

Because `m` and `s` are already present in `a.units`, these types are accepted.

Under revision 2.0, these ordinary named units are catalog/source identities. They must be supplied by the analyzed program/module environment according to the normal declaration/import/prelude rules. The compiler must not silently inject a private SI catalog that is absent from the normative model.

This is especially dangerous because the hard-coded table is only a tiny, arbitrary subset of the actual standard-library unit catalog and can therefore make semantic availability depend on historical compiler shortcuts.

---

# Bug 2 — arbitrary physical units receive an unauthorized synthetic dimension

## Affected code

`internal/sema/units.go`

```go
func defaultUnitDimension(name string, category UnitCategory) Dimension {
    switch category {
    case RatioUnit:
        return Dimension{Base: map[string]int{}}
    case InformationUnit:
        return dimensionFromBase("information", 1)
    default:
        return dimensionFromBase(name, 1)
    }
}
```

The `default` branch covers both:

- `currency`;
- `other`;
- **physical**.

`internal/sema/analyzer.go` then invokes it while registering every unit:

```go
dimension := defaultUnitDimension(stmt.Name.Value, category)
```

Thus:

```sec
unit Temperature physical
```

silently receives a semantic dimension equivalent to:

```text
[Temperature^1]
```

even though revision 2.0 does not authorize such a synthesized physical dimension.

The same occurs for a physical unit whose `impl` never supplies `Dimension`.

## Why this matters

A fabricated dimension becomes real semantic evidence.

It can influence:

- compatibility;
- multiplication and division;
- structural normalization;
- result inference;
- conversion registration;
- comparison;
- overload identity;
- named-type dimensions;
- diagnostics.

The compiler has therefore invented source-language meaning rather than preserving "dimension unknown/not supplied".

---

# Bug 3 — hard-coded unit names can leak stale semantics into source declarations

The registration pass contains:

```go
if existing, ok := a.units[stmt.Name.Value]; ok {
    category = existing.Category
    dimension = existing.Dimension
    ...
}
```

For names present in `builtinUnits()`, a source declaration inherits the compiler's private predefinition before source metadata is applied.

For example, a declaration using a historically hard-coded identifier can begin analysis with a dimension not derived from that declaration or its imported catalog entry.

Later explicit `Dimension` metadata can overwrite it, but omitted metadata leaves the hidden compiler semantics in place.

This creates name-dependent behavior:

```text
unit Foo physical
```

and:

```text
unit Hertz physical
```

do not begin with equivalent revision-2 semantics merely because one identifier happens to occur in `builtinUnits()`.

Ordinary unit identity must not have magic spelling-dependent semantics unless a current normative rule explicitly declares that unit compiler-known.

---

# Required correction

## 1. Remove the implicit ordinary-unit catalog from Sema

`NewAnalyzer()` must not make ordinary units such as:

```text
m
mm
s
kg
Hz
rpm
inch
...
```

available merely because their spellings occur in compiler source.

The active unit registry should be populated from the canonical semantic program environment:

- source unit declarations;
- imported modules/catalogs;
- any explicitly normative compiler-known unit identity, if such a concept is later introduced.

Do not silently retain the current hard-coded list as an undocumented prelude.

## 2. Separate category defaults explicitly

Replace the current catch-all `defaultUnitDimension()` behavior with category-specific rules matching revision 2.0.

Conceptually:

```text
ratio       -> []
information -> [information^1]
currency    -> unique currency axis
other       -> unique semantic axis
physical    -> no synthesized dimension
```

The physical case must preserve the fact that no Dimension has been established.

## 3. Represent "physical Dimension not established"

Do not encode "unknown" as a fabricated named axis.

The semantic representation needs to distinguish at least:

```text
known dimensionless []
known dimension vector [...]
dimension not established
```

This distinction matters because an unknown physical dimension is not equivalent to a unique known base dimension.

If the current `Dimension` structure cannot represent that state, extend `UnitDefinition` or the resolved quantity facts with an explicit validity/known flag.

## 4. Prevent unit algebra from using unresolved physical dimensions

Until a physical unit's Dimension is established by valid metadata/catalog semantics, operations requiring dimensional proof must not treat a fabricated axis as evidence.

Depending on the owning diagnostic rule, this should result in either:

- an incomplete-unit declaration diagnostic where completeness is required; or
- a focused "dimension not established" diagnostic when an operation requires dimensional reasoning.

Do not guess.

## 5. Preserve valid category defaults

The correction must not remove the revision-2 defaults that are actually normative:

- omitted numeric carrier -> `decimal`;
- omitted category -> `other`;
- ratio -> dimensionless;
- information -> `information^1`;
- currency -> unique currency semantic axis;
- other -> unique semantic axis;
- omitted Transform -> `linear`;
- omitted Status -> `active`.

## 6. Load the standard-library unit catalog through normal compiler-owned module/catalog semantics

The existing stdlib unit catalog can provide `m`, `s`, `kg`, `Hz`, and other standard identities.

That source/catalog data, not a second private table in Sema, should be authoritative.

Sema may cache resolved catalog facts, but it must not maintain a divergent hidden semantic catalog.

---

# Required regression tests

Add or revise tests covering at least:

1. undeclared arbitrary unit factor is rejected;
2. undeclared `m` is rejected when the standard unit catalog is not in the analyzed semantic environment;
3. undeclared `s`, `kg`, `Hz`, and `rpm` behave the same way;
4. importing/loading the standard units catalog makes its declared units available;
5. user-defined ordinary unit names do not acquire semantics from spelling alone;
6. `unit Item other` without Dimension receives its unique other-axis default;
7. `unit EUR currency` without Dimension receives a unique currency axis;
8. `unit Ratio ratio` without Dimension is dimensionless;
9. `unit Octet information` without Dimension receives `information^1`;
10. `unit Temperature physical` without Dimension does **not** receive `[Temperature^1]`;
11. two physical units without established Dimension are not considered dimensionally compatible merely through fabricated axes;
12. a physical unit with explicit Dimension uses exactly that Dimension;
13. a standard-library physical unit obtains Dimension from its canonical catalog declaration/metadata rather than `builtinUnits()`;
14. a unit identifier formerly present in `builtinUnits()` does not receive different semantics from another ordinary identifier solely because of its spelling.

---

# Governance correction

> **Applied 2026-08-23:** The hidden ordinary-unit catalog was removed. Unit
> identity now comes from analyzed declarations/catalogs, category defaults are
> explicit, and physical units without Dimension retain an unresolved dimension
> state until metadata establishes it.

The current `frontend.units-v2` governance entry correctly lists implemented omitted-Dimension defaults for:

- ratio;
- information;
- currency;
- other.

However its `required_tests` also contains:

```text
all five categories and their omitted Dimension defaults resolve correctly
```

That wording conflicts with units revision 2.0 because `physical` has no compiler-synthesized omitted-Dimension default.

Replace that test requirement with separate category-specific expectations and explicitly verify that physical Dimension is **not synthesized**.

The existing P1-P12/Sec-MLIR implementation status is not implicated by this correction unless a lowering path consumes the fabricated frontend unit fact.
