# MD-040, MD-046, MD-047 and MD-056 — Unit Qualification, Register Bit Extraction and Rounding

- **Status:** Applied 2026-10-13 to `rules/types/units.md` ("Qualified unit references", "Explicit rounding", "Interaction with registers and FFI", conversion sections), `rules/declarations/registers.md` (§ 12.2), `rules/types/types.md` ("Explicit low-bit extraction"), `rules/foundations/grammar.md` (impl, BitExtractionExpression suffix, rounded conversions, qualified unit references), `rules/declarations/impl.md`, `rules/foundations/names_scopes_visibility.md`, `rules/library/core-library.md` (§ 10.3) and `rules/types/temporal.md`; all core, stdlib and fixture sources use `<Unit>`; parser, Sema, formatter and LSP implement the four decisions. Remaining (governance): Semantic IR/lowering of unit quantities and rounded conversions, rounding to decimal/floating carriers, the stdlib/math rulebook (§ 5.7), and the platform migration that enforces `try` for bare Sec-only conversions (MD-056). New open question MD-062 (value types of non-text unit metadata).
- **Created:** 2026-10-13
- **Last updated:** 2026-10-13
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/corrections/applied/md040-md046-md047-md056-units-register-bits-rounding-correction-20261013.md`
- **Decision IDs:** `MD-040`, `MD-046`, `MD-047`, `MD-056`
- **Primary rulebooks:** `rules/types/units.md`, `rules/declarations/registers.md`, `rules/foundations/grammar.md`, `rules/types/types.md`
- **Also affected:** `rules/declarations/impl.md`, `rules/foundations/names_scopes_visibility.md`, `rules/platform/hardware-register-access.md`, `rules/foundations/operators.md`, `rules/library/core-library.md`, the compiler-known member/operation documentation, the relevant `stdlib/math` rulebook, and platform/FFI conversion call sites
- **Related correction:** `md015-time-unit-duration-conversion-correction-20261013.md` (MD-015); its duration and time-quantity rules reuse the unit-resolution and rounding decisions here
- **Supersedes:** `md046-md056-bit-extraction-correction-20261012.md` **in full**. Its `MD-046` unit-source exclusion and unresolved-unit status are replaced by the complete rules below. Avoid installing both corrections as independent competing normative sources.
- **Replaces:** No existing published paragraph identifiers. Amend canonical paragraphs in place; never renumber stable paragraph references.
- **Governance:** Reconcile existing per-book implementation-status fragments and tests; do not duplicate IDs. Removing an item from the authoritative latest `missing-decisions.yaml` requires the owning rulebooks and affected governance to be synchronized and verified.

---

## 1. Scope and decision authority

**1.1.** This document records four **already approved** Sec language decisions. It is not an invitation to re-decide them and is not an audit or a replacement for `missing-decisions.yaml`.

**1.2.** The four decisions solve, respectively: ambiguity between a unit and an ordinary type with the same spelling; unit-safe observations and assignments of register bit fields; explicit rounding of numeric/unit conversions; and deliberate low-bit extraction from signed and unsigned integer representations.

**1.3.** The related MD-015 duration conversion is already documented by its separate correction. Integration of these corrections should be coordinated, not merged into newly invented independent conversion models.

## 2. MD-040 — Mandatory `<Unit>` qualification

**2.1.** A reference to a unit **always** uses the `<...>` unit-reference spelling, whether or not there is an ordinary type of the same name. A unit declaration retains the existing `unit Name ...` declaration syntax.

**2.2.** Canonical source forms:

```sec
unit Volt physical

type Volt uint

impl <Volt> {
    LongName: "Volt"
    Symbol: "V"
    Kind: electric_potential
    Dimension: [mass^1, length^2, time^-3, electric_current^-1]
    Scale: 1
    System: SI
}

impl Volt {
    fn IsZero() bool {
        return self == 0
    }
}
```

**2.3.** The two `Volt` declarations may coexist in their distinct unit and ordinary-type namespaces in the same module. `impl <Volt>` resolves only the unit; `impl Volt` resolves only the ordinary type. Resolution must not depend on declaration order, fallback lookup or a hard-coded preference for types over units.

**2.4.** A call to a unit conversion and a call to a unit static member likewise require explicit unit qualification:

```sec
let converted := <Volt>(value)
let metadata := <Volt>.Symbol
```

The corresponding ordinary type forms retain their existing meanings:

```sec
let converted := Volt(value)
let bytes := Volt.SizeOf
```

`<Volt>.Symbol` denotes the unit's declared symbol metadata, while `Volt.SizeOf` denotes the ordinary type's compiler-known storage-size member. Neither form changes the other's namespace.

**2.5.** Unit annotations in quantity and numeric-carrier type references continue to use their established forms, for example `uint16<Volt>`, `decimal<Volt>`, `<Volt>`, and `5<s>`. `unit Volt physical` remains a declaration, not a reference. Any legacy unqualified unit conversion or unit-method example, such as `rpm(hz)` or `impl rpm`, must migrate to `<rpm>(hz)` and `impl <rpm>`.

**2.6.** Explicit unit-conversion functions retain the existing contract that they must be declared and validated. Qualified lookup changes their **source spelling**, not their mathematical conversion or its compatibility checks.

## 3. MD-056 — Exact `bit[N](value)` operation

**3.1.** The existing contextual `bit[N]` register-field representation gains **one additional expression form**:

```sec
bit[N](value)
```

This is a compiler-known **low-bit extraction operation**, not a new `bit[N]` variable type, a family of `uintN` types, a new keyword, or an ordinary fallible numeric cast. `let x: bit[12]` remains invalid in ordinary storage declarations.

**3.2.** Bit zero is the least-significant bit (LSB), as already defined by the register rules. `bit[N](x)` selects exactly the N lowest numerical bit positions, independent of endian byte order or MMIO field allocation direction.

**3.3.** The required normative formula is:

\[
\operatorname{bit}[N](x) = x \bmod 2^N
\]

Here modulo returns the **non-negative remainder** in `[0, 2^N - 1]`, including for negative `x`. For a signed source, this is equivalent to extracting the low N bits of Sec's defined two's-complement bit pattern. It does not perform a signed value-preserving conversion.

**3.4.** `N` must be compile-time known, must satisfy `1 <= N <= 256`, and must not exceed the source's resolved integer bit width. Invalid constants, out-of-range widths, and widths exceeding the operand's representation are **compile-time errors**. The width of target-sized `int`/`uint` comes from the active target profile, not the compiler host.

**3.5.** The operation is safe, pure, infallible, non-allocating, and needs neither `try` nor `unsafe`. There is no automatic sign extension, zero extension or widening when N exceeds the source width.

**3.6.** The result is always an **ordinary unsigned integer** of the smallest existing fixed width able to hold N bits:

| N (inclusive) | Exact result type |
|---|---|
| 1–8 | `uint8` |
| 9–16 | `uint16` |
| 17–32 | `uint32` |
| 33–64 | `uint64` |
| 65–128 | `uint128` |
| 129–256 | `uint256` |

`uint8`, `byte`, and `char` have the same eight-bit storage domain but are semantically distinct; the result for N in 1–8 is specifically `uint8`. No `uint12` or `uint24` type is introduced.

**3.7.** The compiler records the exact proven result interval `0..2^N-1`, irrespective of whether the physical result carrier has additional unused high bits. An ordinary explicit numeric conversion can use that proof when deciding whether it may succeed without `try`.

**3.8.** Signed and unsigned integer-family operands are supported, including `byte` and `char` as eight-bit sources. Named nominal types, unrelated enums, pointers, floating-point values, or foreign `c::` ABI types are not implicitly unwrapped. A separately specified explicit projection must be used where applicable. **Unit-bearing integer operands are the specifically approved exception governed by § 4.**

**3.9.** Valid examples:

```sec
let word: uint32 := 0xABCD1234
let low4 := bit[4](word)      // uint8,  0x04
let low12 := bit[12](word)    // uint16, 0x234
let low24 := bit[24](word)    // uint32, 0xCD1234

let negative: int64 := -1
let low8 := bit[8](negative)  // uint8,  255
let low64 := bit[64](negative) // uint64, 0xFFFFFFFFFFFFFFFF
```

**3.10.** These are distinct operations; the first deliberately extracts a bit pattern, while the second asks for a value-preserving numeric conversion:

```sec
let word: uint16 := ReadWord()
let low := bit[8](word)  // uint8; explicit low-bit extraction
let checked := try uint8(word) // Checked numeric conversion
```

Do not introduce `BitCast`, a special syscall cast, or an alternative wrapping constructor as a consequence of this rule. Same-width signed-to-unsigned reinterpretation is already achievable by `bit[W](signedValue)` when W is a supported source width.

**3.11.** Invalid cases include `bit[0](value)`, `bit[257](value)`, runtime-varying N, and `bit[16](source)` when `source` has an eight-bit integer representation. Error diagnostics must report both the requested and available widths where possible.

## 4. MD-046 — Unit-annotated register fields and bit extraction

**4.1.** Register-field width and the unit identity are **independent**. A field `bit[N]<U>` occupies N physical register bits and has a unit contract `<U>`. N must remain within the width limits of its enclosing register and the applicable register specification.

**4.2.** Reading a plain unsigned `bit[N]` field gives the smallest supported unsigned numeric carrier covering N bits. Reading `bit[N]<U>` gives that **same carrier with the unit preserved**. In particular:

| Register field | Read value type |
|---|---|
| `bit[4]` | `uint8` |
| `bit[12]` | `uint16` |
| `bit[24]` | `uint32` |
| `bit[12]<rpm>` | `uint16<rpm>` |
| `bit[24]<Hz>` | `uint32<Hz>` |

The compiler also retains the stronger field-derived range `0..2^N-1`. Neither the read nor a write may discard declared hardware permissions or access semantics.

**4.3.** A field write must prove **both** the required unit identity and the representable N-bit value range. A wider value must never be silently truncated. A register field assignment does not become an implicitly fallible operation; unknown or out-of-range source values require explicit handling before the assignment.

**4.4.** A `bit[N]` extraction **never inherits a unit automatically**. The approved optional suffix is specific to the new bit extraction expression:

```sec
bit[N](value)<U>
```

Without the suffix, a unit-bearing integer source is explicitly reduced to a **unitless** unsigned result. With the suffix, the extracted result bears the explicitly named `<U>` identity. The numeric bit result is identical in both forms; no unit-scale transform, affine offset, logarithm or rounding is performed.

**4.5.** If the source already has a unit, the suffix may preserve **that same semantic unit** only. It must not reinterpret the source as a different unit, even if its numeric carrier and physical dimension happen to match. Changing units uses the ordinary, validated explicit unit-conversion machinery, such as `<Hz>(speed)` where a valid conversion is declared.

**4.6.** If the source has **no** unit, `bit[N](raw)<U>` explicitly labels that raw bit result with `<U>`. This is the hardware/protocol boundary operation needed to interpret unsigned raw data. It is the programmer's explicit assertion of the unit identity, **not** proof of a physical source-unit conversion.

**4.7.** Representative behavior:

```sec
let raw: uint16 := 0x1ABC
let frequency: uint16<rpm> := ReadSpeed()

let plain := bit[12](raw)               // uint16; unitless
let speed := bit[12](raw)<rpm>          // uint16<rpm>
let bare := bit[12](frequency)          // uint16; original unit explicitly discarded
let retained := bit[12](frequency)<rpm> // uint16<rpm>
let invalid := bit[12](frequency)<Hz>  // ERROR: unit reassignment without conversion
```

**4.8.** Writing to an annotated field therefore has a precise form:

```sec
unit rpm uint physical

type Motor register[16] {
    Speed: bit[12]<rpm>,
    _: bit[4],
}

let raw: uint16 := ReadRawSpeed()
let speed: uint16<rpm> := ReadSpeed()

motor.Speed = bit[12](raw)<rpm>
motor.Speed = bit[12](speed)<rpm>
// motor.Speed = bit[12](raw)  // ERROR: missing unit identity.
```

The example illustrates value/unit checks on writes. Actual register access remains subject to the registered platform's MMIO and access-mode rules.

**4.9.** **No automatic unit change is permitted** through `bit[N](value)<U>`. For example, if `speed` is measured in rpm, `<Hz>(speed)` is a mathematical unit conversion; `bit[12](speed)<Hz>` is a compile-time error. A program that deliberately strips a unit through `bit[N](speed)` has explicitly opted into low-level behavior; the compiler cannot infer erased provenance from an unrelated stored integer later.

**4.10.** The expression grammar admits the optional `<UnitExpression>` **only after** `bit[N](expression)`; it does not, by itself, authorize a general `AnyCall()<U>` construct. Parser/formatter must disambiguate the unit suffix from ordinary comparison operators, preserve whitespace canon, and keep the grammar of register field declarations unchanged.

## 5. MD-047 — One explicit rounding model

**5.1.** No unit or numeric conversion may silently round, truncate, overflow or otherwise lose precision. The programmer may request an explicit rounding mode during a valid conversion to a target numeric carrier; the conversion must still satisfy all other type, dimension, kind, range and named-type contracts.

**5.2.** Numeric core owns **one** canonical public enum `RoundingMode`, available without importing `stdlib/math`. Its entire declaration (documentation before members) is:

```sec
/**
 * Selects the rounding policy for explicit numeric conversions.
 *
 * Description:
 *   No mode is applied implicitly.
 */
enum RoundingMode {
    // Rounds to the nearest representable value; exact ties select an even result.
    NearestEven

    // Rounds to the nearest representable value; exact ties move away from zero.
    NearestAwayFromZero

    // Rounds toward zero.
    TowardZero

    // Rounds away from zero.
    AwayFromZero

    // Rounds toward positive infinity.
    Ceil

    // Rounds toward negative infinity.
    Floor
}
```

**5.3.** An explicit rounding argument is accepted as an additional argument to the existing typed numeric/unit conversion form; **no new keyword or standalone rounding operator** is added:

```sec
let speed: decimal<rpm> := 100
let hz := try uint16<Hz>(speed, RoundingMode.Ceil)
```

The example assumes a valid predefined explicit conversion between `rpm` and `Hz`. Without a corresponding declared/validated conversion, rounding cannot manufacture unit compatibility. The same policy form applies to `duration(0.5<ns>, RoundingMode.Ceil)` as specified in MD-015.

**5.4.** Rounding takes place **once**, after evaluating the exact mathematical unit transform (including its scale/offset where defined), to a value representable in the target carrier. Do not round intermediate values and then transform again; this can create double-rounding errors. Logarithmic/affine details still belong to MD-048.

**5.5.** Without an explicit mode, an exact representable conversion succeeds. A statically known inexact or out-of-range conversion is rejected at compile time; a dynamic checked conversion uses the existing `ConversionError` policy. With an explicit mode, the selected loss of fractional precision is permitted but other errors (such as out-of-range output) are not silently ignored. `try` is required for a conversion that remains fallible under existing `Result` semantics; it is not required solely because a rounding mode appears in the source when no failure is possible.

**5.6.** Exact mathematical meanings for integral targets:

| Mode | 1.6 | -1.6 | Halfway 2.5 | Halfway -2.5 |
|---|---:|---:|---:|---:|
| `NearestEven` | 2 | -2 | 2 | -2 |
| `NearestAwayFromZero` | 2 | -2 | 3 | -3 |
| `TowardZero` | 1 | -1 | 2 | -2 |
| `AwayFromZero` | 2 | -2 | 3 | -3 |
| `Ceil` | 2 | -1 | 3 | -2 |
| `Floor` | 1 | -2 | 2 | -3 |

**5.7.** `RoundingMode` belongs in core's numeric API, **not** `core/error.sec`. An ordinary `math.Round()` operation belongs to `stdlib/math` and consumes the shared enum. Its complete overloaded signature family, if not already normative in the stdlib/math rulebook, must be specified **there**; MD-047 does not invent unspecified public overloads in a compiler rulebook.

**5.8.** Conversion errors use the already approved core `ConversionError` (MD-012), including `PrecisionLoss` for a failed exactness requirement and `OutOfRange` for an unrepresentable destination where those variants apply. Do not add `RoundingError` or another competing type just for unit conversions.

## 6. Grammar and compiler requirements

**6.1.** Amend the existing grammar, without introducing a parallel lexer dialect, with these new/clarified forms:

```text
UnitImplTarget
    ::= "<" UnitExpression ">"

QualifiedUnitCall
    ::= UnitAnnotation "(" [ ArgumentList ] ")"

QualifiedUnitMemberAccess
    ::= UnitAnnotation "." Identifier

BitExtractionExpression
    ::= Contextual("bit") "[" IntegerConstant "]"
        "(" Expression ")" [ UnitAnnotation ]

RoundedNumericUnitConversion
    ::= NumericTypeReference [ UnitAnnotation ]
        "(" Expression "," RoundingModeExpression ")"
```

These productions name required semantic forms. Integrate them using the current canonical expression/precedence and type-reference productions; do not duplicate an existing `UnitAnnotation` or override the existing ordinary call grammar. The `RoundingModeExpression` operand resolves to the actual core `RoundingMode` enum rather than forming a special token.

**6.2.** The formatter writes `impl <U>`, `<U>(value)`, `<U>.Member`, `bit[N](value)`, and `bit[N](value)<U>` canonically. It does not insert whitespace between the extraction expression and its optional suffix. Existing configurable indentation/line width rules apply.

**6.3.** Semantic IR must preserve unit identity, source and destination numeric carriers, physical bit source width, the proven extraction interval, and the explicit rounding policy until all legality/proof checks and lowering are complete.

**6.4.** Error messages explain the cause and resolution: ambiguous or unqualified unit access, invalid extraction width, unsigned return mapping, unsafe or incorrect field writes, unit mismatch on raw-bit output, and precision/overflow of conversions. Reuse existing canonical diagnostic IDs when available; no new IDs are allocated by this correction.

## 7. Conformance and negative tests

**7.1. MD-040:** Test same-module `unit Volt` plus `type Volt`, both `impl <Volt>` and `impl Volt`, qualified unit calls and members, and rejection of legacy bare unit-call/member syntax without weakening the independent namespaces. Test unit references even when no same-named ordinary type exists.

**7.2. MD-056:** Test all carrier-width transitions N = 1, 8, 9, 16, 17, 32, 33, 64, 65, 128, 129, 256 with operands having adequate physical bit widths; test signed `-1`, positive, zero, `byte`/`char`, target-sized sources, and endian independence. Reject widths 0, 257, nonconstant, greater than source width, or invalid operand types.

**7.3. MD-046:** Test `bit[N]` field read/write interval proof; `bit[N]<U>` field read returning unsigned-carrier `<U>` and proven N-bit range; explicit suffix assignment from raw input; stripping and explicitly retaining an existing source unit; rejection of `bit[N](rpmValue)<Hz>`; rejection of implicit unit creation and implicit truncation. Preserve register access-mode behavior and device/platform error behavior.

**7.4. MD-047:** Test each rounding mode on positive, negative, halfway and integer values; checked dynamic and compile-time-constant conversions; overflow and unit incompatibility despite rounding; no double rounding; consistency with `duration` nanosecond precision from MD-015; presence of the public enum once, and math package integration without compiler import dependency.

**7.5.** Test formatter, LSP hover, diagnostic rendering, frontend, Semantic IR, backend and incremental/cold compilation equivalence where applicable. No rulebook correction is evidence of implementation completeness by itself.

## 8. Integration and closure

**8.1.** Integrate the four decisions into their owning canonical rulebooks and update preexisting example syntax rather than retaining conflicting examples. In particular, migrate bare unit conversion/impl syntax, preserve register field grammar, and extend it only with the explicit `bit[N](value)` expression and optional unit suffix.

**8.2.** The earlier `md046-md056-bit-extraction-correction-20261012.md` is superseded here, because it excluded unit-bearing operands and marked MD-046's unit contract undecided. Do not preserve those exclusions or the open-status statement when integrating.

**8.3.** Reconcile established governance fragments with actual compiler and core status. Update one canonical item per existing identifier; do not report unimplemented features as implemented. Add/adjust conformance tests for newly decided semantics.

**8.4.** Once rulebook paragraphs, tests and governance are synchronized, **MD-040, MD-046, MD-047 and MD-056 are eligible for removal from the latest authoritative `missing-decisions.yaml`**. Their *normative decisions* are complete now. **MD-015** has its separate correction and is likewise eligible upon integration. **MD-048** (affine/logarithmic unit transforms) and other unrelated open decisions are not resolved by this document.
