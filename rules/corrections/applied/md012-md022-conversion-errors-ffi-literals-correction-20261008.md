# MD-012 and MD-022 — Checked Conversion Errors and C Scalar Literal Shaping

- **Status:** Applied 2026-10-08 to `rules/types/contracts.md`, `rules/types/types.md`, `rules/errors/errorhandling.md`, `rules/errors/runtime_checks.md`, `rules/platform/ffi.md` § 8 and `sec/core/error.sec`; runtime checked-conversion typing and C character literal shaping are tracked in `governance/types.yaml` and `governance/ffi.yaml`
- **Created:** 2026-10-08
- **Last updated:** 2026-10-08
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Proposed canonical path:** `rules/corrections/pending/md012-md022-conversion-errors-ffi-literals-correction-20261008.md`
- **Decision IDs:** `MD-012`, `MD-022`
- **C qualification:** every `C::` spelling in this correction was migrated to `c::` on 2026-10-09 by MD-020 (`rules/corrections/applied/md020-md021-md023-md024-ffi-c-bindings-null-correction-20261008.md` § 6.6); the semantics are unchanged
- **Amends:** `rules/types/contracts.md`, `rules/types/types.md`, `rules/errors/errorhandling.md`, `rules/errors/runtime_checks.md`, `rules/platform/ffi.md`
- **Canonical source owning the new public declarations:** `sec/core/error.sec` (`module core`)
- **Related decision:** `MD-043` (`char`/`byte` domain and `rune` literal default)
- **Replaces:** None; do not renumber existing normative paragraphs when applying.

## 1. Decision scope

1.1. `MD-022` is resolved by defining contextual shaping for `c::bool` and C character scalars while preserving explicit conversions between distinct Sec/C scalar types.

1.2. `MD-012` is resolved by one public error channel, `ConversionError`, for checked conversion, with independently meaningful `ContractError` for named-type contract violations.

1.3. All three supporting public declarations — `ContractKind`, `ContractError`, and `ConversionError` — belong in `sec/core/error.sec`, using its existing `module core`. They must not be independently redeclared in the compiler, stdlib, or another core source file. They are core declarations visible by the normal core visibility rules without a user import.

1.4. The existing `OverflowError`, `RangeError`, `PrecisionError`, `EncodingError`, `MathError`, and other existing core errors are **not** removed or aliased to `ConversionError`. Their existing use by other operations is unchanged.

## 2. MD-022 — Contextual C scalar literal shaping

2.1. An untyped boolean literal `true` or `false` can be shaped to `c::bool` when the target type is known. This is literal shaping, not an implicit conversion from a previously typed `bool` value.

```sec
let a: bool := true
let b: c::bool := true
let c: c::bool := false

let source: bool := true
let invalid: c::bool := source   // Invalid: different nominal scalar type.
let converted := c::bool(source) // Valid explicit conversion.
```

2.2. A single-quoted character literal defaults to `rune` as determined by MD-043. In an explicit `c::char`, `c::schar`, or `c::uchar` target context, the *literal* may instead be shaped to that C scalar when the decoded Unicode scalar's **numeric value** is representable by that target's C ABI integer range.

2.3. Literal shaping does not perform UTF-8, Latin-1, locale, platform-codepage, or other text encoding. The scalar's code-point number is the candidate integer value. The active ABI model controls `c::char` signedness and all C character scalar limits. No first-byte-of-UTF-8 fallback is permitted.

```sec
let a: c::char := 'A'    // Valid on an ABI that represents 65.
let b: c::schar := 'B'   // Valid when 66 is representable.
let c: c::uchar := 'é'   // Valid when 233 is representable.

// These are compile-time errors when the target C scalar cannot
// represent the decoded code-point number:
let invalidA: c::char := 'é'    // Invalid on signed 8-bit c::char.
let invalidB: c::uchar := 'π'   // Invalid on 8-bit c::uchar.
```

2.4. A typed `rune`, typed `char`, typed `bool`, or typed `c::` scalar does **not** implicitly change to another nominal `c::` scalar through assignment or argument passing, even when the underlying representations coincide. Use the target-type conversion syntax.

```sec
let a: c::int := 42
let invalid: c::long := a
let b := c::long(a) // Checked if the complete source domain does not fit.
```

2.5. Explicit C-to-C, C-to-Sec and Sec-to-C conversions use the established whole-source-domain proof rule: if the complete source type fits, the conversion is infallible; otherwise a runtime-dependent conversion uses the checked conversion mechanism and may require `try`. Compile-time-known nonrepresentable values are rejected during semantic analysis.

2.6. Character literal shaping is **not** a grant of general implicit scalar conversion. The stricter `char` integer-literal rule from MD-043 remains: `let c: char := 65` is invalid and `65t` is the explicit numeric char literal. Ordinary representable integer literal shaping for `c::` numeric targets remains as already defined by FFI § 8.

## 3. MD-012 — Canonical public declarations

3.1. The complete public surface introduced by this decision is the following exact set. All declarations are owned by `sec/core/error.sec`, alongside existing errors. Comments must immediately precede what they document, including each public variant, so the LSP can display the actual declarations and associated documentation.

```sec
/**
 * Identifies the kind of named-type contract that rejected a value.
 */
enum ContractKind {
    // The value violates a range bound.
    Range

    // The value is absent from an allowed-membership set.
    In

    // The value is not odd.
    Odd

    // The value is not even.
    Even

    // The value is not a multiple of the declared divisor.
    MultipleOf

    // The value is not finite.
    Finite

    // The value does not match the declared regex contract.
    Regex

    // The value is shorter than the minimum length.
    MinLen

    // The value exceeds the maximum length.
    MaxLen

    // The value does not have the exact required length.
    ExactLen

    // The value is empty where a nonempty value is required.
    NotEmpty

    // The value contains non-unique direct elements.
    Unique
}

/**
 * Describes a failure of a declared named-type contract.
 *
 * Contract identity is an ordinal in the declaring type, not a runtime
 * pointer, an allocated message, or a globally stable identifier.
 */
type ContractError union error {
    // A named-type contract rejected the value.
    Violation {
        // Category of the contract that failed.
        Kind: ContractKind

        // Zero-based source-order index of the failing contract.
        DeclarationIndex: uint
    }
}

/**
 * Describes a failure of an otherwise valid checked type conversion.
 */
type ConversionError union error {
    // The value is outside the destination scalar's representable range.
    OutOfRange

    // The value is not a valid destination scalar value, such as a
    // surrogate or out-of-domain Unicode scalar in a rune conversion.
    InvalidScalar

    // The value cannot be represented exactly at the destination precision.
    PrecisionLoss

    // A required destination decimal scale is not representable.
    ScaleOverflow

    // A non-finite input is disallowed by the destination's intrinsic domain.
    NonFinite

    // A declared named-type contract rejected an otherwise representable value.
    Contract(ContractError)
}
```

3.2. `ContractKind` is a plain enum, not itself an error. It is kept in `sec/core/error.sec` because it is part of the public `ContractError` payload. `ContractError` and `ConversionError` are the two new public error types.

3.3. The declaration shapes and names above are normative. Implementation must not replace them with prose-only compiler metadata, omit variants, merge the error types, or silently add variants on the grounds that existing narrower core errors have similar names.

## 4. Checked conversion semantics

4.1. A missing legal conversion relation is a compile-time type error, not a `ConversionError` variant.

4.2. For a legal conversion, an intrinsic destination-domain or representation failure takes precedence over declared contracts. The primitive phase can produce the appropriate `ConversionError` variant.

4.3. After a value has passed intrinsic conversion validation, its declared named-type contracts are checked **in source order**. The first violated contract yields `ConversionError.Contract(ContractError.Violation { ... })` with the corresponding `Kind` and zero-based `DeclarationIndex`.

4.4. A `finite` contract violation returns a `ContractError.Violation` with `Kind: ContractKind.Finite`. `ConversionError.NonFinite` is reserved for intrinsic destination-domain restrictions, not used to disguise an ordinary user-declared `finite` contract.

4.5. A statically known invalid conversion or contract violation is a compile-time diagnostic where the established compile-time-checking rules require one. `ConversionError` is the runtime checked-conversion error channel; defining it does not demote proven compile-time errors into runtime results.

4.6. The declaration index identifies an ordinal **within the named-type declaration**. It is not promised stable across source edits that insert, remove, or reorder contracts. Diagnostics should use preserved source locations where available, rather than inventing pointers or source strings in the public error payload.

4.7. These error types introduce no implicit allocation, reflection, exception, or GC behavior. They preserve typed `Result` and explicit `try` use for potentially failing operations.

4.8. A conversion that is proven infallible does not need a `Result` solely because the general checked-conversion error family exists. A runtime-dependent checked conversion that can fail exposes `ConversionError` as its error type under the owning conversion rules.

## 5. Examples and diagnostics

5.1. A contract failure follows the canonical layering:

```sec
type Percent int range 0..100

fn Convert(value: int) Result[Percent, ConversionError] {
    let percent := try Percent(value)
    return Ok(percent)
}
```

5.2. For a runtime value of `150`, the underlying integer representation is valid but the range contract fails. The semantic result is the `Contract` variant containing `Violation`, `Kind: ContractKind.Range`, and `DeclarationIndex: 0`. The exact typed-construction and pattern syntax for binding individual union payloads remains governed by the existing union/match rulebooks; this correction does not invent new grammar there.

5.3. Diagnostics must distinguish an undefined conversion relation, an intrinsic domain failure, and a failed declared contract, and identify the destination type and the offending value when the value is known.

5.4. Compiler tests must cover exact widening, checked narrowing, invalid Unicode scalar conversions, decimal precision/scale failures, intrinsic non-finite rejection, every `ContractKind`, first-violation source ordering, compile-time-known failures, `c::bool` literal shaping, C character ABI signedness, and rejection of implicit typed `c::` conversions.

## 6. Required rulebook and code synchronization

6.1. Add the exact declarations from § 3 to `sec/core/error.sec`. Preserve all existing core error declarations. Keep canonical documentation directly adjacent to the public declarations and each member.

6.2. Update `rules/types/contracts.md` to specify `ContractKind`, `ContractError`, zero-based declaration indices, and first-failing-contract semantics. Do not duplicate these declarations elsewhere as competing sources of truth; reference the canonical core source.

6.3. Update `rules/types/types.md`, `rules/errors/errorhandling.md`, and `rules/errors/runtime_checks.md` so intrinsic conversion failures and contract failures use `ConversionError` consistently and compile-time diagnostic precedence is preserved.

6.4. Update `rules/platform/ffi.md` § 8 to specify `c::bool`, `c::char`, `c::schar`, and `c::uchar` literal shaping; keep §§ 5–9's nominal type identity and ABI-compatibility requirements intact.

6.5. Synchronize compiler registration, Semantic IR, diagnostics, core API documentation, tests, and the appropriate governance fragments. Do not claim implemented status before verified tests establish it.

6.6. Mark `MD-012` and `MD-022` resolved by accepted normative decisions, then remove the missing-decision entries only when owning rulebooks and governance are synchronized according to the missing-decisions file's removal policy.

6.7. Preserve all existing stable section identifiers when integrating this correction into the owning books.
