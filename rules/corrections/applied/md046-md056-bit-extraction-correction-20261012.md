# MD-046 / MD-056 — Explicit Low-Bit Extraction with `bit[N](value)`

- **Status:** Applied 2026-10-12 to `rules/types/types.md` ("Explicit low-bit extraction"), `rules/declarations/registers.md` (§ 4, § 12.1), `rules/foundations/grammar.md` ("Bit extraction expression"), `rules/foundations/operators.md` (bitwise cross-reference) and `rules/platform/hardware-register-access.md` (§ 37.3); MD-056 is closed, MD-046 stays open for unit-annotated fields (§ 7.3), and enforcing the try requirement for Sec-only conversions with the platform migration to `bit[N]` remains tracked in governance
- **Created:** 2026-10-12
- **Last updated:** 2026-10-12
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path when accepted:** `rules/corrections/pending/md046-md056-bit-extraction-correction-20261012.md`
- **Decision IDs:** `MD-046` (non-unit bit-field writes and explicit width restriction), `MD-056` (explicit extraction of integer bit patterns)
- **Primary rulebooks:** `rules/declarations/registers.md`, `rules/types/types.md`, `rules/foundations/grammar.md`
- **Also amends:** `rules/foundations/operators.md`, `rules/platform/hardware-register-access.md` (integration of source bit ranges only), and related compiler-known operation documentation
- **Related rulebooks:** `rules/types/units.md`, `rules/platform/ffi.md`, `rules/platform/abi.md`
- **Related decision still open:** `MD-046` — the complete conversion contract for *unit-annotated* `bit[N]<unit>` register fields; the present correction does **not** define an implicit removal of units or a new unit-conversion spelling.
- **Replaces:** No published numbered paragraph. Update existing paragraphs in place without changing published section/paragraph identifiers.
- **Governance:** Update the existing implementation-status entries; do not duplicate them. A missing-decision item is removed only after its entire owning rulebook contract and affected governance are synchronized.

## 1. Purpose and boundaries

**1.1.** This correction records the language owner's decision to reuse Sec's existing `bit[N]` syntax in **one additional expression form**:

```sec
bit[N](value)
```

This is a **compiler-known explicit low-bit extraction operation**, not a new keyword, a new general cast operator, a function in `core`, a standalone `bit[N]` type, or a family of `uintN` types.

**1.2.** Existing `bit` and `bit[N]` declarations remain register-field representations as specified by `rules/declarations/registers.md` § 4. `let value: bit[12]` remains invalid as an ordinary variable declaration. Likewise, bit-backed enum declarations and nominal register declarations retain their existing semantics and identities.

**1.3.** `bit[N](value)` is a deliberate, visibly lossy **bit-pattern extraction**. It is different from an ordinary checked numeric conversion, such as `uint8(value)`, whose purpose is value preservation. No implicit truncation is introduced by this correction.

**1.4.** No new `BitCast`, `uint12`, `uint24`, `uintN`, or `unsafe` requirement is introduced. The accepted operation works independently of any system-call ABI and does not depend on hardware memory layout.

## 2. Normative operation and mathematical definition

**2.1.** Let `x` be the source integer value, interpreted according to Sec's defined fixed-width, two's-complement bit semantics for signed integers. The result selects bit positions `0` through `N - 1`, inclusive, from the source bit representation. **Bit 0 is the least-significant bit (LSB)**, as already defined by `rules/declarations/registers.md` § 6.

**2.2.** The normative value semantics are:

\[
\operatorname{bit}[N](x) = x \bmod 2^N
\]

Here `mod` is defined to produce the **unique non-negative remainder** in the interval `0..2^N-1`, including when `x` is negative. This explicitly avoids differences between programming languages' signed-remainder operators.

**2.3.** Extraction operates on a numeric bit pattern, **not** on ordered bytes in memory. It is unaffected by the target's endian order, `lsb-first`/`msb-first` register-field allocation, byte swapping, physical MMIO access width, or C ABI representation.

**2.4.** The operation is pure, safe, infallible and does not allocate. It requires neither `try` nor `unsafe`. Backends may lower it to masking or an equivalent semantics-preserving operation, but backend instruction availability does not alter Sec semantics.

**2.5.** No high-bit extension is performed. `bit[N](value)` requires the source operand's known representation width to be **at least N**. A narrower source must not be silently zero-extended or sign-extended to make the request valid.

## 3. Width domain and return type

**3.1.** `N` is a positive compile-time-known integer in the range **1 through 256**, inclusive. In particular, `bit[0](...)` and `bit[257](...)` must be compile-time errors. A non-constant width is also a compile-time error.

**3.2.** `N` may not exceed the source operand's resolved physical integer representation width. For the target-sized `int` and `uint` families, the width comes from the active target profile. An unavailable or ambiguous width is diagnosed rather than guessed from the host system.

**3.3.** The return type is **always unsigned**. Choose the smallest *existing* fixed-width Sec unsigned integer type capable of representing N bits:

| Requested width `N` | Return type |
|---|---|
| 1–8 | `uint8` |
| 9–16 | `uint16` |
| 17–32 | `uint32` |
| 33–64 | `uint64` |
| 65–128 | `uint128` |
| 129–256 | `uint256` |

**3.4.** The return type is chosen **from N**, not from the original operand type or signedness. `bit[8](int64Value)` and `bit[8](uint64Value)` both have type `uint8`. `bit[12](uint16Value)` has type `uint16`; no `uint12` type is added.

**3.5.** The compiler preserves a proven value interval for the expression:

```text
0 <= result <= 2^N - 1
```

This may establish that a later explicit numeric conversion is safe and does not require `try`. A wider return carrier does **not** make the unused upper bits significant or remove the interval fact.

**3.6.** `uint8`, `byte` and `char` share an eight-bit unsigned representation but remain semantically distinct type names according to the previously approved MD-043 rules. In particular, `bit[N](value)` returns **`uint8`**, not contextually `char` or `byte`, when `N` is in `1..8`.

## 4. Accepted source operands

**4.1.** The defined operand domain in this revision comprises Sec's ordinary primitive signed and unsigned integer types, including target-sized `int`/`uint` when their widths are resolved, and the eight-bit `byte` and `char` scalars. For `byte` and `char`, extraction reads their eight-bit numeric representation. This is not text decoding.

**4.2.** The operation does not implicitly unwrap a named nominal type, unit-bearing value, enum, register, pointer, foreign `c::` scalar, string or rune. Those sources require their own separately defined conversion or projection before bit extraction; the compiler must not silently discard their type contracts. In particular, this correction does not introduce a way to strip a unit annotation.

**4.3.** A request of width `N` exceeding the operand width is invalid even if the operand's current mathematical value would fit in N bits. The request is for **N available source bit positions**, not a value-range test.

## 5. Exact source syntax and examples

**5.1.** The canonical call-like expression is written without spaces within `bit[N]`:

```sec
let word: uint32 := 0xABCD1234

let low4 := bit[4](word)     // uint8,  value 0x04
let low8 := bit[8](word)     // uint8,  value 0x34
let low12 := bit[12](word)   // uint16, value 0x234
let low16 := bit[16](word)   // uint16, value 0x1234
let low24 := bit[24](word)   // uint32, value 0xCD1234
let low32 := bit[32](word)   // uint32, value 0xABCD1234
```

**5.2.** Signed values are handled identically by low-bit extraction, using the non-negative modulo definition rather than signed numeric narrowing:

```sec
let negative: int64 := -1

let low4 := bit[4](negative)    // uint8,  15
let low8 := bit[8](negative)    // uint8,  255
let low12 := bit[12](negative)  // uint16, 4095
let low64 := bit[64](negative)  // uint64, 0xFFFFFFFFFFFFFFFF
```

**5.3.** The explicit distinction from checked numeric conversion is:

```sec
let wide: uint16 := 0xABCD

let truncated := bit[8](wide)    // uint8, 0xCD; explicit truncation
let checked := try uint8(wide)   // Fails: 0xABCD does not fit uint8
```

`try` is shown for the checked conversion of a value that is not statically known to fit. For statically known out-of-range constants, normal compile-time rejection still applies.

**5.4.** The extractor can establish the proof needed by an ordinary checked conversion:

```sec
let wide: uint16 := ReadValue()
let narrow := uint8(bit[8](wide))  // Valid; fit is compiler-proven.
```

The extra `uint8(...)` is redundant in this exact example because `bit[8](wide)` already returns `uint8`; it remains valid as an explicit illustration of the two distinct operation kinds. The formatter must not be required to insert such a conversion.

**5.5.** The following are invalid:

```sec
let source: uint8 := 255

let none := bit[0](source)       // ERROR: N must be >= 1.
let tooWide := bit[9](source)    // ERROR: N exceeds source width 8.
let huge := bit[257](source)    // ERROR: N exceeds maximum 256.
let asType: bit[4]              // ERROR: bit[N] is not a standalone value type.
```

**5.6.** If `N` is a compile-time constant expression in the permitted register-width syntax, the same evaluation and diagnostics apply. This correction introduces no general runtime-width `bit[n](x)` variant.

## 6. Register integration: unannotated fields

**6.1.** For a register field declared `bit[N]` without unit annotations, the field width remains its physical layout width. An explicit `bit[N](source)` expression is a valid way to provide a value known to fit that field's representable unsigned interval. The result is still the ordinary unsigned carrier selected by § 3, **not** a standalone `bit[N]` value.

```sec
type Sensor register[16] {
    Reading: bit[12],
    _: bit[4],
}

let source: uint16 := ReadValue()
sensor.Reading = bit[12](source)
```

**6.2.** The register assignment above is legal because the source's explicit bit extraction proves `0..4095` and the destination field accepts that width's representable values. The compiler must not require a second lossy conversion or an invented `uint12` type. The register's existing read/write permissions, special write behavior, shadow contract and platform operation-plan requirements still apply.

**6.3.** A write of an unproven wider integer value to `bit[N]` must **not silently truncate**. An ordinary direct write is valid only when the existing field conversion rules prove the value fits; otherwise the programmer must explicitly restrict the bits or use a separately defined checked conversion. This correction does not make field assignment itself fallible.

**6.4.** Bit extraction cannot be used to bypass field-level access protections, write-one-clear semantics, write-zero-clear semantics or device-specific hardware contracts.

## 7. Unit-annotated register fields: preserve the open boundary (MD-046)

**7.1.** A declaration such as `Speed: bit[12]<rpm>` denotes **both** a 12-bit physical field and an independent unit contract. `bit[N](x)` only defines bit selection and the returned unsigned integer type. It does not perform a unit conversion, supply `<rpm>`, or convert between unit carriers.

```sec
type Motor register[16] {
    Speed: bit[12]<rpm>,
    _: bit[4],
}
```

**7.2.** Therefore this correction must not be interpreted as permission to write a unitless `bit[12](raw)` into `Speed` by silently discarding its unit contract. Equally, it must not invent implicit unit removal when extracting bits from a unit-bearing source.

**7.3.** The remaining MD-046 question is **normative**: the exact field observation type for `bit[N]<unit>`, how its unit-bearing carrier and narrowed range interact on reads and writes, and the **existing or subsequently approved explicit syntax** for a checked or provably safe unit-bearing field write. Preserve the current implementation's behavior until a separate decision resolves these points. Do **not** remove MD-046 from `missing-decisions.yaml` solely on the basis of this correction.

**7.4.** This does not weaken the already decided overarching rule: **no silent truncation or loss of a unit contract** is permitted as final normative behavior.

## 8. Grammar and parser integration

**8.1.** Add one contextual call-shaped expression form under the current source grammar, alongside ordinary call and conversion expressions. In EBNF-style notation, using the register grammar's existing constant-width concept:

```text
BitExtractionExpression
    ::= Contextual("bit") "[" IntegerConstant "]" "(" Expression ")"
```

**8.2.** This is a new **expression use** of existing spelling, not a declaration, general type conversion, ordinary function call or user-definable generic. Lexer recognition of contextual `bit` remains consistent with current `BitFieldType` syntax.

**8.3.** Extend AST/Sema/compiler-known operation identification, integer-width validation, signed modulo semantics, result-type resolution and proven range facts. No general `bit[N]` variable type may become legal as a side effect of parsing this operation.

**8.4.** The formatter prints `bit[N](expression)` canonically, without separating the `bit` token, brackets or call parentheses by invented spaces; normal formatting of the inner expression still applies.

## 9. Diagnostics

**9.1.** Compile-time errors must clearly explain at least:

1. `N == 0`, negative or otherwise non-positive N;
2. `N > 256`;
3. `N` is not a valid compile-time integer constant;
4. source type is not an accepted integer/byte/char source;
5. `N` exceeds the resolved source representation width;
6. use of `bit[N]` as a standalone variable type;
7. use of an unproven over-width value in register field assignment;
8. attempts to lose a unit annotation or bypass nominal type conversion.

**9.2.** A diagnostic must identify the requested N, the source width when known, and a remedy such as selecting a permitted width, explicitly extracting fewer bits, or using the normal checked numeric conversion when **value preservation** rather than extraction is intended.

**9.3.** Existing published diagnostic IDs remain stable. The implementer must reuse the appropriate canonical diagnosis where defined; this correction does not allocate new diagnostic IDs.

## 10. Required regression and conformance coverage

**10.1.** Add positive frontend, semantic IR and backend cases for:

- N at each unsigned return-type boundary: `1, 8, 9, 16, 17, 32, 33, 64, 65, 128, 129, 256`, with matching source widths;
- positive values, zero, and negative values (including `-1`) within fixed-width signed integer source types;
- `byte`/`char` eight-bit inputs, yielding `uint8` rather than reusing their nominal type;
- `bit[8](uint16Value)` and `bit[12](uint16Value)` returning different carrier widths;
- expressions with values that prove narrower bounds than the return type width;
- `bit[N]` used as an argument to a legal plain `bit[N]` register-field assignment;
- endian-independent extraction under big- and little-endian targets;
- target-sized `int`/`uint` source-width resolution from `CompilationPlan`.

**10.2.** Add negative cases for:

- `bit[0]`, `bit[257]`, non-constant width, and requests above source width;
- booleans, floating-point, pointer, string, rune, nominal enum, register and unit-bearing operands without a valid explicit projection;
- standalone storage declarations of `bit[N]`;
- implicit direct register writes that could truncate;
- any regression that silently strips a unit annotation from a `bit[N]<unit>` field.

**10.3.** Verify that checked ordinary conversions still reject potentially non-representable values and that explicit bit extraction never introduces a `try`, panic, heap allocation or unsafe requirement.

## 11. Integration and status instructions

**11.1.** Amend `rules/declarations/registers.md` § 4 and § 12 to distinguish **field representation** `bit[N]` from **extraction expression** `bit[N](value)` and to permit proven-fit plain-field assignments without silent truncation. Preserve its § 6 LSB numbering and § 8 endianness distinction as already normative.

**11.2.** Amend `rules/types/types.md` hardware layout and conversion sections to enumerate the new operation separately from ordinary checked `TargetType(value)` conversions. Add exact return-type and width rules rather than saying only that the operation is bit-oriented.

**11.3.** Amend `rules/foundations/grammar.md` with the call-shaped expression production, preserving declaration parsing. Amend `rules/foundations/operators.md` to cross-reference the operation without treating it as a normal infix bitwise operator.

**11.4.** Align compiler/LSP hover/documentation and diagnostics with the normative operation. Because the form is compiler-known syntax, do **not** invent a source-level `fn bit` or pseudo-`init` in core.

**11.5.** Update existing affected governance fragments and tests, verifying real implementation before marking any item implemented. The explicit bit-pattern extraction portion of MD-056 is decided here; MD-056 must not be removed until its other stated requirements, if any, and affected rulebook/governance synchronization are complete. **MD-046 remains partially unresolved** as specified by § 7.

**11.6.** Do not rewrite the `missing-decisions.yaml` file from within this correction. Its owners should update entries when they apply and verify this normative change.
