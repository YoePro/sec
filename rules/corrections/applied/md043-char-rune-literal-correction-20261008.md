# MD-043 — `char`, `byte`, and `rune` literal semantics

- **Status:** Applied 2026-10-08 to `rules/types/types.md`, `rules/foundations/lexical_structure.md` (§ 12.7, § 13) and `rules/foundations/operators.md`; frontend implemented and tests synchronized 2026-10-08 (`governance/types.yaml` `frontend.types-core`, `frontend.literal-family-suffix-v2`); MD-043 closed
- **Created:** 2026-10-08
- **Last updated:** 2026-10-08
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Proposed canonical path:** `rules/corrections/pending/md043-char-rune-literal-correction-20261008.md`
- **Decision ID:** `MD-043`
- **Amends:** `rules/types/types.md`, `rules/foundations/lexical_structure.md`, `rules/foundations/operators.md`
- **Related, not resolved here:** `MD-022` (C scalar literal shaping), `MD-012` (checked conversion failure API)
- **Replaces:** None. Preserve existing owning-rulebook section numbering and references when integrating.

## 1. Scope and authority

1.1. This correction defines the semantic distinctions among Sec's `byte`, `char`, and `rune` scalar types and changes the default type of a single-quoted character literal from `char` to `rune`.

1.2. It does not define or rename any `rune` methods or properties. The `rune` core API, including any symbolic-constant functionality, remains owned by its actual declarations. Do not invent a `ConstantValue` or `ToConstantValue` signature in response to this correction.

1.3. The rules below govern source semantics. Equal machine representation must not erase type identity.

## 2. Scalar types and domains

2.1. `byte` represents an unsigned 8-bit data byte, with the complete value domain `0..255`.

2.2. `char` represents a character-oriented 8-bit scalar, also with the complete value domain `0..255`.

2.3. `byte` and `char` are **distinct Sec types** with potentially different members and intended uses, even though they use identical 8-bit representations.

2.4. `rune` represents a Unicode scalar value: `U+0000..U+10FFFF`, excluding surrogate values `U+D800..U+DFFF`. It is semantically distinct from `char` and `byte`.

2.5. A `char` value is not inherently UTF-8 encoded. Its raw value can participate in byte-oriented encodings, including Latin-1, when an explicitly chosen encoding interprets it. No ambient locale or encoding is inferred from a `char` value.

2.6. Ordering of `char` values is unsigned numeric order `0..255`. It is not Unicode collation, locale collation, or alphabetic order. Ordering of `rune` values follows Unicode scalar numeric order.

## 3. Exact literal syntax and typing

3.1. A single-quoted character literal denotes exactly one Unicode scalar after escape decoding. **Its default type is `rune`**, independently of the scalar's numerical value.

```sec
let letter := 'a'       // rune, U+0061
let pi := 'π'           // rune, U+03C0
let omega := 'Ω'        // rune, U+03A9
let accent := 'é'       // rune, U+00E9
```

3.2. A default `rune` literal does not turn into `char` merely because its value is in `0..255`. An explicitly expected `char` may context-shape a single-quoted literal only if its decoded Unicode scalar value is in `0..255` and the existing contextual character-literal rule applies. Contextual literal shaping is not an implicit `rune`-to-`char` conversion of a previously typed expression.

```sec
let letter: char := 'a'  // contextual char literal, value 97
let accent: char := 'é'  // contextual char literal, value 233
let omega: rune := 'Ω'  // rune, U+03A9
```

3.3. Contextual `char` construction from `'Ω'` is invalid because `U+03A9` is greater than 255.

```sec
let invalid: char := 'Ω' // compile-time error: scalar does not fit char
```

3.4. The numeric literal suffix `t` selects `char` exactly; `r` selects `rune` exactly. Both apply to integer-form numeric literals only.

```sec
let first: char := 65t
let second: char := 0x41t
let third: rune := 65r
let fourth: rune := 0x03C0r
```

3.5. `t`-suffixed literals must be in `0..255`. A `t` literal above 255 is invalid even when it is a valid Unicode scalar. This replaces any rule allowing all Unicode scalar values for `t`-suffixed numerics.

```sec
let maximum: char := 255t
let invalid: char := 256t   // compile-time error
let invalid2: char := 300t  // compile-time error
```

3.6. An unsuffixed integer literal is **not** implicitly shaped to `char`. Character intent must be expressed with a `t` suffix, a single-quoted literal with a valid expected `char` context, or an explicit conversion. This is a deliberate exception to integer-literal contextual shaping for integer-family types.

```sec
let correct: char := 65t
let alsoCorrect: char := char(65)
let incorrect: char := 65 // compile-time error: use 65t or char(65)
```

## 4. Explicit conversions

4.1. `byte(charValue)` and `char(byteValue)` are explicit, total, value-preserving conversions over `0..255`. The compiler may implement them without a machine instruction. They must not imply that `byte` and `char` are interchangeable for assignment or overload resolution.

```sec
let character: char := 65t
let raw: byte := byte(character)
let restored: char := char(raw)
```

4.2. Runtime `rune`-to-`char` conversion requires a range check when it cannot be proven statically. The public error/result syntax for a failing conversion is **not specified here**; it must remain consistent with the checked-conversion decision tracked as `MD-012`. A compiler-known out-of-range constant is rejected at compile time.

4.3. This correction does not decide implicit or explicit conversions between Sec `char` and the target ABI's `C::char`, `C::schar`, or `C::uchar`. Those are separate C scalar types and are covered by `MD-022`.

## 5. Required changes to existing rulebooks

5.1. In `rules/types/types.md`, replace `## char` text saying character literals default to `char`. State that single-quoted literals default to `rune`, clarify the 8-bit `char`/`byte` domains and type identities, and retain the existing `t` and `r` numeric suffixes.

5.2. In `rules/foundations/lexical_structure.md` § 12.7, replace the combined Unicode-scalar validation of `t` and `r` with **two distinct checks**: `t` in `0..255`, `r` in the Unicode scalar domain excluding surrogates.

5.3. In `rules/foundations/lexical_structure.md` § 13, replace “It is a `char` by default” with “It is a `rune` by default”. Keep the single-decoded-scalar lexer invariant and existing lexical diagnostics unchanged.

5.4. In `rules/foundations/operators.md`, change `char` ordering wording from “Unicode scalar value within the valid `char` domain” to “unsigned 8-bit value (0..255)”; preserve explicit `char`/`rune` type separation and contextual literal comparison rules.

5.5. Update examples, syntax references, compiler-known type documentation, formatter fixtures, and regression tests that assumed `let x := 'a'` has type `char`.

5.6. Do not renumber established normative sections. Integrate correction text within the existing sections or append stable numbered subparagraphs.

## 6. Required compiler diagnostics and tests

6.1. Inferred-literal tests must establish that both `let a := 'a'` and `let pi := 'π'` have type `rune`. Character-literal inference must not branch on ASCII, Latin-1, or non-Latin-1 value ranges.

6.2. Verify valid `char` values at 0, 127, 128, 233, and 255, including the `t` suffix and explicit `char(byteValue)` conversion.

6.3. Reject `256t`, `300t`, an out-of-range single-quoted scalar context-shaped to `char`, and `let a: char := 65` with precise compiler diagnostics identifying the domain/type mismatch and a valid repair.

6.4. Reject implicit `byte`/`char` assignment and implicit typed `rune`/`char` assignment; accept explicit representable conversions subject to the owning conversion contracts.

6.5. Verify `char` comparisons order by numeric byte value, without locale or text collation.

6.6. Do not reuse an unrelated existing diagnostic ID. Use the diagnostics registry when assigning a stable ID; this document specifies the required failure but does not invent an ID.

## 7. Deliberately separate follow-up

7.1. The user's `rune` symbolic-constant functionality (including examples involving `'π'`) is an existing core-API concern, **not a reason to introduce a new language literal form**. This correction makes `'π'` a `rune` by default, so the normal `rune` member surface can apply.

7.2. Before documenting the precise symbolic-constant call or property, inspect the current authoritative `sec/core/rune.sec` version used by the developer; do not infer the name, signature, supported constants, or returned numeric type from examples alone.

7.3. `MD-043` can close after this correction is integrated and tests are synchronized. `MD-022` and `MD-012` remain separately tracked.
