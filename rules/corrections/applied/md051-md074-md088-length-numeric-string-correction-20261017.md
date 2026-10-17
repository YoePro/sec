# Sec Language Correction: Numeric-to-String Conversion and Unsigned Lengths

| Field | Value |
|---|---|
| Status | Applied 2026-10-17 — integrated into `rules/library/core-library.md` §§ 1.4 and 15, `rules/types/types.md` ("Numeric-to-text conversion"), `rules/compiler/compiler_known_members.md` ("Global `len`", Semantic IR distinctions, numeric `ToString()`, migration notes), `rules/collections/collections.md` § 3.1, `rules/analysis/pitfall_analysis.md` ("Final-element access requires non-empty proof"), the slice, dynamic-array and string-core sync notes and the P14/P16/P18/P23 package notes and TODOs. Compiler: `len` returns `uint` (registry and inference); `string(numericValue)` is `Result[string, StringError]`; pitfall analysis recognizes `len(x) - 1`. Tests `TestLenIsUintAndNumericStringConversionIsFallible`, `TestCompilerKnownLenReturnsUintForStringsAndArrays`, `TestPitfallAnalysisRequiresNonEmptyProofForFinalElement`. Governance `frontend.types-core`, `sema.pitfall-analysis`. MD-074 and MD-088 removed from `missing-decisions.yaml`; MD-051 also removed 2026-10-17 after the owner confirmed that `byte` counts as `uint8` (`string(b)` equals `b.ToString()`), with numeric-to-named-string conversion split out as MD-093; runtime numeric-text lowering remains a known hole. Known holes (verified 2026-10-17): no route lowers global `len(...)` or numeric text yet; predecessor/reverse-loop pitfalls and underflow-versus-bounds explanations are not implemented. |
| Created | 2026-10-17 |
| Updated | 2026-10-17 |
| Revision | 1.0 |
| Sec version | 0.1 |
| Canonical path | `rules/corrections/applied/md051-md074-md088-length-numeric-string-correction-20261017.md` |
| Replaces | No existing canonical rulebook; supersedes conflicting statements in the affected rulebooks once applied |
| Decision IDs | MD-051, MD-074, MD-088 |
| Owners | `rules/library/core-library.md`; `rules/types/types.md`; `rules/compiler/compiler_known_members.md`; `rules/collections/collections.md`; `rules/analysis/pitfall_analysis.md`; Sec MLIR P16/P18/P23 and slice sync where they declare signed `len` |

> Integration note: This correction records the design decision. It does not assert that the compiler, core, MLIR, tests, or governance already implement or validate it. Preserve the numbered section references in later revisions; append new sections instead of renumbering existing sections.

## §1. Scope and governing principle

1. A sequence length is a nonnegative count. Sec represents lengths with `uint`, not `int`.
2. The compiler-known `len(value)` and the corresponding compiler-known `.Len` property must denote the same logical count and must both have result type `uint` wherever both forms are defined.
3. This correction changes only the *result type*, not the existing set of supported operands for `len` or the definition of logical length. It does not add new collection overloads implicitly.
4. The same change resolves MD-074 (cross-package `len` representability) and MD-088 (slice signed-length representability). There is no conversion to a signed target-width integer inside `len`.

## §2. Exact compiler-known `len` declarations

The canonical declarations in `rules/library/core-library.md` §1.4 must become:

```sec
/**
 * Returns the Unicode scalar count of a valid UTF-8 string.
 *
 * Returns:
 *   The nonnegative scalar count, as uint.
 */
fn len(value: string) uint

/**
 * Returns the element count of an owning sequence.
 *
 * Parameters:
 *   - value: The array to inspect; no ownership is consumed.
 *
 * Returns:
 *   The nonnegative element count, as uint.
 */
fn len[T](value: T[]) uint

/**
 * Returns the element count of a shared array view.
 *
 * Parameters:
 *   - value: A non-consuming shared array view.
 *
 * Returns:
 *   The nonnegative element count, as uint.
 */
fn len[T](value: ref T[]) uint

/**
 * Returns the element count of a mutable array view.
 *
 * Parameters:
 *   - value: A non-consuming mutable array view.
 *
 * Returns:
 *   The nonnegative element count, as uint.
 */
fn len[T](value: ref mut T[]) uint
```

No user overload or redeclaration of `len` is introduced. All existing evaluation-once, no-allocation, and non-consuming guarantees remain unchanged. `len(string)` continues to count Unicode scalar values, not encoded bytes; `string.SizeOf` remains its UTF-8 payload-byte count.

## §3. Representability, targets, and indexing

1. A legitimate `uint` sequence length must not be reduced, saturated, wrapped, rejected, or converted to `int` merely because `len` was called.
2. Compiler, Semantic IR, Sec MLIR, LLVM, and other backend representations must preserve this unsigned width and the exact length value. A target may still enforce its independently defined storage/object-size constraints; such constraints are not invented by this correction.
3. No new implicit `uint` ↔ `int` conversion is introduced. Code requiring a signed offset must use an explicit checked conversion according to the existing numeric conversion rules. The conversion can fail when the value does not fit the signed representation.
4. Existing signed-index support and bounds checks are not changed. A length and an index are distinct concepts: the length is always nonnegative, while intermediate index/offset arithmetic may legitimately use signed numbers.
5. This revision does not broaden the legal element index domain and does not specify a new panic or error type.

## §4. Checked underflow is distinct from bounds failure

Sec integer subtraction is checked. For an empty collection:

```sec
let lastIndex := len(values) - 1
```

`len(values)` is zero; subtracting one from `uint(0)` is **unsigned underflow**. The arithmetic fails *before* any indexing operation. It must not wrap to `uint.max`.

Conversely, where a signed index expression yields `-1`, using it to index a collection is a separate, invalid bounds access. Do not describe the unsigned case as "indexing at -1" or require an index-bounds panic when no indexing has occurred.

The existing `rules/foundations/operators.md` policy for checked arithmetic and deterministic arithmetic-failure paths remains authoritative, including its current limitations on the final runtime failure representation. This correction introduces neither a new panic contract nor an implicit Result return from `len`.

## §5. Pitfall analysis: last element and predecessor

Extend the existing `rules/analysis/pitfall_analysis.md` section **"Final-element access requires non-empty proof"** rather than creating a duplicate pitfall family. Use canonical `.Len`/`len(...)` spelling in new examples.

```sec
// Potential checked unsigned underflow if values is empty.
let last := values[len(values) - 1]
```

A safe, nonallocating form is:

```sec
if values.Len > 0 {
    let last := values[values.Len - 1]
    Process(last)
}
```

The same pitfall applies to predecessor access (`i - 1` when `i == 0`) and reverse loops. The analysis must consider dominating guards and control-flow proofs rather than warning solely on the textual presence of `- 1`.

- If invalid arithmetic or bounds are statically proven, the owning arithmetic or bounds diagnostic takes priority. Pitfall analysis enriches the explanation and possible fix, rather than producing an overlapping error.
- If safety cannot be proven or disproven, a configurable pitfall finding may explain the missing nonempty/positive-index proof according to the existing confidence and severity model. Do not invent a new mandatory diagnostic ID without updating the diagnostic registry.
- Suppress this finding when a valid dominating nonempty/index-positive proof exists.
- Explain whether the hazard is unsigned underflow or a bounds violation; they are not interchangeable.

## §6. Migration and source compatibility

1. Update `rules/library/core-library.md` §1.4, `rules/collections/collections.md` §3.1, all compiler-known registries and exact Sec declarations, and the P16/P18/slice synchronization rulebooks that depend on `len -> int`.
2. Audit core, stdlib, platform code, examples, diagnostics, tests, and generated source that uses `len(...)` in an `int`-typed position or compares it with signed offsets. Migrate intentionally; do not insert unchecked implicit casts.
3. Do not silently reinterpret existing `int` indexes as `uint`. Compiler diagnostics should explain the new `len` result type and show when an explicit checked conversion or nonnegative guard is needed.
4. Keep the existing logical-count versus byte-size separation for strings and collections.

## §7. MD-051: numeric text conversion forms

1. `.ToString()` is the preferred, already defined presentation of numeric-to-text materialization.
2. An explicit `string(numericValue)` conversion is also permitted, but must use exactly the same default formatting contract and exact fallible result as that numeric type's `ToString()` with no format argument. It is not a character-code-point conversion.
3. Both forms retain the `Result[string, StringError]` contract and its allocation/error obligations. Neither permits implicit conversion of an already typed numeric value to `string` in an unrelated expression, assignment, or argument.
4. Format-specific numeric presentation continues to use `.ToString(format)`; the `string(...)` form does not create a second formatter or a formatting-argument overload.
5. The existing frontend permission for `string(numericValue)` must not be treated as conforming until Sema, diagnostics, core/runtime implementation, and tests consistently enforce these rules, including named numeric wrappers where their existing conversion contracts permit that route.

Illustrative usage:

```sec
let number := 123

let preferred := try number.ToString()
let explicit := try string(number)
```

Both must yield the same text. The implicit assignment `let text: string := number` remains invalid. This rule does not change the distinct `char` and `rune` conversions.

## §8. Required tests

1. `len`/`.Len` have exactly type `uint` and identical logical values for each supported owning array, shared slice, mutable slice, and string.
2. `len` is evaluated exactly once, has no allocation or ownership-transfer effect, and yields correct `uint` values beyond signed maximum where a valid target test representation can express such a length.
3. No signed overflow, truncation, wrapping, saturation, undocumented panic, or implicit conversion appears when lowering `len` to Semantic IR, MLIR, or LLVM.
4. An explicit checked `int(len(value))` conversion follows existing conversion error behavior on unrepresentable lengths.
5. Empty `values`: `len(values)-1` takes the checked arithmetic-failure path; no index operation runs first.
6. Nonempty guarded access succeeds; an unguarded last-element access is explained by pitfall analysis without duplicate error reporting.
7. Tests for reverse iteration, predecessor access at `i==0`, signed offsets, and byte lengths versus scalar lengths exercise the correct types and boundaries.
8. Numeric `.ToString()` and explicit `string(numericValue)` agree in default formatting and error behavior, including allocation failure; numeric-to-string implicit coercion is rejected.
9. Both optimized and unoptimized builds preserve the same arithmetic/bounds behavior.

## §9. Governance and closure

- Add or update canonical implementation/governance entries by existing IDs; synchronize, do not duplicate legacy implementation records.
- MD-074 and MD-088 are one semantic decision with independent package implementation/test references. Do not close either until its affected owning books and governance are synchronized.
- MD-051's design is recorded here, but close it only after the runtime and compiler behavior match the common conversion contract.
- Record partially implemented and unvalidated states accurately; neither specification acceptance nor a passing isolated test alone establishes whole-system validation.
