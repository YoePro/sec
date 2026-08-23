# Correction 7 — enum-to-integer narrowing is incorrectly treated as infallible

## Audit context

- Repository: `github.com/YoePro/sec`
- Repository baseline: `c515862`
- Audited: `2026-08-19`
- Primary rulebook: `rules/declarations/enums.md`
- Document revision: **1.0**
- Created: `2026-08-13`
- Last updated metadata: `2026-08-13`
- Later repository synchronization: the rulebook was touched again on `2026-08-16`
- Reviewed rulebook baseline: `ae227c1`

## Classification

Implementation bug.

This is not a language-rule change.

The canonical enum rule requires enum-to-integer conversion to use the normal checked integer-conversion rules. If the target integer type cannot represent every possible runtime enum value, and the actual value is not statically known, the conversion is fallible.

Current Sema special-cases checked conversion **to** an enum but not conversion **from** an enum to an integer. Every allowed enum-to-integer conversion is therefore typed as the target integer directly, even when narrowing can fail.

Package 11's implemented enum-to-integer operation is not itself contradicted by this correction: its documented fixtures are intentionally inside a Sema-proven infallible conversion boundary. The bug is that frontend Sema does not actually establish that boundary for enum-to-integer conversions.

---

## Normative requirement

`rules/declarations/enums.md` revision 1.0 states:

```text
The numeric value represented by the enum is converted under the normal checked
integer conversion rules.

If the target integer type cannot represent every possible runtime value and the
actual value is not statically known, the normal fallible conversion rules apply.
```

Therefore Sema must distinguish at least:

1. statically known enum value that fits the target integer;
2. statically known enum value that does not fit;
3. runtime enum whose complete semantic domain fits the target integer;
4. runtime enum whose semantic domain does not fit the target integer.

Cases 1 and 3 are provably infallible.

Cases 2 and 4 must follow the canonical checked integer-conversion semantics.

---

# Bug 1 — `inferCallAsConversion()` only performs checked enum analysis when the target is an enum

## Affected code

`internal/sema/analyzer.go`

The conversion path resolves source and target types and allows the conversion:

```go
if !canExplicitConvert(targetType, valueType) {
    ...
}
```

It then special-cases only:

```go
if targetType.Kind == EnumType {
    conversionType, valid := a.enumConversionResultType(
        targetType,
        valueType,
        expr.Arguments[0],
    )
    ...
    return conversionType, ...
}
```

Every other conversion immediately returns:

```go
return targetType, ...
```

Thus:

```sec
uint8(valueOfWideEnum)
```

is always inferred as `uint8`, never as a fallible conversion, regardless of the enum's possible runtime values.

---

# Bug 2 — `canExplicitConvert()` permits enum-to-integer without range semantics

The generic conversion predicate contains:

```go
if isIntegerType(target) && value.Kind == EnumType {
    return true
}
```

This is correct only as a statement that the **explicit conversion form exists**.

It is not sufficient to determine:

- representability;
- compile-time failure;
- runtime fallibility;
- required `try`;
- resulting `Result` type;
- lowering requirements.

Current `inferCallAsConversion()` treats this simple permission result as if it also proved infallibility.

---

# Example — closed enum narrowing

```sec
enum Wide uint16 {
    SMALL = 1,
    LARGE = 300,
}

fn Narrow(value: Wide) uint8 {
    return uint8(value)
}
```

`Wide` is an ordinary closed enum.

Its possible runtime numeric value classes are:

```text
1
300
```

`uint8` cannot represent `300`.

Therefore Sema cannot prove `uint8(value)` infallible for an arbitrary runtime `Wide`.

The conversion must follow the normal fallible checked integer-conversion rule and cannot simply have type `uint8`.

Current Sema returns `uint8` directly.

---

# Example — statically known enum member

```sec
let a := uint8(Wide.SMALL)
let b := uint8(Wide.LARGE)
```

These should be handled using the normal checked integer-conversion rules with the actual statically known enum value.

The compiler must not classify both conversions identically merely because their source semantic type is `Wide`.

`Wide.SMALL` is representable as `uint8`.

`Wide.LARGE` is not.

The precise diagnostic/result behavior must be the same as the canonical checked integer conversion of the corresponding known integer value; this correction does not invent a separate enum error family for enum-to-integer narrowing.

---

# Open bit-backed enums

For:

```sec
enum DeviceMode bit[16] {
    OFF = 0,
    ACTIVE = 1,
}
```

the semantic runtime domain is the complete unsigned 16-bit domain, not just the declared members.

Therefore conversion of an arbitrary runtime `DeviceMode` to `uint8` is not provably infallible.

Conversely, conversion to an integer type that represents the complete `0..65535` domain can be proven infallible.

The proof must use the enum's semantic domain:

- ordinary closed enum -> unique declared numeric value classes;
- open `bit[N]` enum -> complete N-bit unsigned domain.

It must not infer the domain solely from the machine representation.

---

# Required correction

## 1. Add enum-to-integer conversion result analysis

When:

```text
source.Kind == EnumType
target is an integer type
```

Sema must run a dedicated checked-conversion analysis instead of returning `targetType` immediately.

The analysis should consume the resolved enum semantic facts and the target integer representability facts.

## 2. Handle statically known enum values

If the source enum value is known at compile time:

1. obtain the resolved semantic numeric value;
2. apply the normal checked integer-conversion rule;
3. accept directly when representable;
4. produce the canonical compile-time checked-conversion diagnostic when not representable.

Do not route enum-to-integer narrowing through `EnumValueError`.

`EnumValueError` belongs to validation of integer values **as enum values**.

Enum-to-integer conversion follows the normal integer conversion error semantics.

## 3. Prove runtime conversion infallibility from the complete enum domain

For an ordinary closed enum:

- normalize numeric aliases into unique runtime value classes;
- prove every possible numeric value representable by the target integer.

For an open `bit[N]` enum:

- use the complete unsigned N-bit domain;
- declared members alone are insufficient proof.

If every possible runtime enum value fits, the conversion remains a direct target integer.

If not, use the normal fallible checked integer-conversion result/type and require ordinary `try` handling where canonical conversion rules require it.

## 4. Preserve Package 11's infallible boundary

Package 11 already has:

```text
OpEnumToInteger
```

for Sema-proven infallible conversions.

Keep that operation and implemented status for conversions for which Sema has actually proven success.

Do not silently emit `OpEnumToInteger` for a conversion that can fail.

## 5. Add explicit Semantic IR for the fallible case

When enum-to-integer conversion is potentially fallible, Semantic IR must preserve:

- source enum identity/domain;
- target integer type;
- checked-conversion semantics;
- success value;
- canonical integer-conversion failure;
- source provenance.

The exact operation may reuse the canonical checked integer-conversion representation if that representation already exists or is introduced elsewhere.

Do not create an enum-specific narrowing error when the source rule delegates to normal checked integer conversion.

## 6. Lower failure control flow explicitly

Sec MLIR/later lowering must:

- emit direct enum-to-integer conversion only when Sema proves it infallible;
- otherwise lower through the canonical checked integer conversion and Result/try control flow;
- never silently truncate or reinterpret an out-of-range enum numeric value.

---

# Governance reconciliation

The current `frontend.enums` entry records integer-to-enum checked conversion in detail but does not record enum-to-integer narrowing/fallibility as remaining work.

Package 11 currently says:

```text
enum constants, explicit integer-to-enum and enum-to-integer conversion
```

are implemented, while also explicitly stating that its fixtures remain inside the:

```text
Sema-proven infallible boundary
```

These statements can remain true if the scope is made explicit:

> Package 11 implements enum-to-integer conversion for conversions that Sema has proven infallible.

The missing frontend work is the proof and fallible path outside that boundary.

Do not downgrade Package 11 as a whole.

---

# Required regression tests

> **Applied 2026-08-23:** Sema now checks known enum members and complete closed
> or open-bit domains against the integer target. Unproven narrowing returns
> `Result[target, ArithmeticError]`, requires ordinary `try`, and exposes a
> fallible resolved conversion fact. Lowering work remains in governance.

Add tests covering at least:

## Closed ordinary enum

1. runtime enum -> same-width/sufficient-width integer is infallible;
2. runtime enum -> narrower integer is infallible when all declared numeric value classes still fit;
3. runtime enum -> narrower integer is fallible when at least one declared numeric value does not fit;
4. numeric aliases do not change the domain proof;
5. large positive enum member prevents an unsigned narrow-target proof;
6. negative enum member prevents conversion to an unsigned target from being considered universally infallible;
7. known fitting enum member converts directly;
8. known non-fitting enum member receives the canonical checked integer-conversion failure diagnostic.

## Open bit-backed enum

9. `bit[8]` enum -> `uint8` is provably infallible;
10. `bit[8]` enum -> wider unsigned integer is provably infallible;
11. `bit[16]` enum -> `uint8` is fallible even if every declared member happens to fit;
12. open-enum proof uses the complete bit domain, not only declared members.

## Cross-layer

13. infallible conversion produces the existing Semantic IR `OpEnumToInteger`;
14. potentially fallible conversion is not emitted as an unconditional `OpEnumToInteger`;
15. potentially fallible conversion requires the canonical `try`/Result handling;
16. failed runtime narrowing cannot silently truncate;
17. Package 11's existing infallible enum conversion fixtures continue to pass.
