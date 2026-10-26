# MD-093 — Numeric Conversion to Named String Types: Normative Correction

**Status:** Applied 2026-10-26 — owning rulebooks synchronized; Sema relation/protected-operand/contract checks and CLI/LSP regressions verified. Runtime text materialization and the accepted two-stage Semantic IR/lowering path remain tracked under P16/P23 in governance/types.yaml.  
**Created:** 2026-10-26  
**Updated:** 2026-10-26  
**Revision:** 1  
**Sec version:** 0.1  
**Canonical path:** `rules/corrections/applied/md093-named-string-conversion-correction-20261026.md`  
**Replaces:** None  
**Decision authority:** Sec language owner, 2026-10-26  
**Decision ID:** `MD-093` (`numeric-to-named-string-conversion`)  
**Decision register:** `missing-decisions.yaml`  
**Owning rulebooks:** `rules/types/types.md`, `rules/library/core-library.md`  
**Applicable existing error rules:** `rules/errors/errorhandling.md` §§ 11–14; `rules/types/contracts.md`, "Conversion failure layers" and "Public conversion and contract errors"  
**Repository review:** Rechecked local `main` at `0240c2005d513652fb65000626c803676ffe1aba` and the current owning-rulebook working tree during integration (2026-10-26). The frontend precheck reproduced acceptance of the invalid direct numeric conversions and rejection of the preferred protected-argument form before the fixes. No remote-main synchronization is claimed.

---

## § 1. Purpose, scope, and authority

**§ 1(1)** This correction resolves **MD-093** for Sec 0.1. It addresses a numeric value being passed directly to the explicit conversion syntax of a **named type whose underlying type is `string`**. It does not introduce a new conversion API, error type, or `try` grammar.

**§ 1(2)** The accepted rule is that `Name(number)` is **invalid**, even when `type Name string` and the same numeric value may be explicitly converted to a plain `string`. An enclosing `try` never legitimizes a conversion relation that does not exist.

**§ 1(3)** The programmer must explicitly perform numeric-to-text materialization, and must handle its fallibility. The preferred combined spelling is `try Name(number.ToString())`. The explicit inner form `Name(try number.ToString())` is also valid, subject to the separate fallibility of the named-type conversion.

**§ 1(4)** This correction is normative until incorporated into the owning rulebooks. Preserve existing paragraph identifiers when incorporating it: amend the relevant existing text or add uniquely numbered paragraphs; never renumber already referenced provisions.

**§ 1(5)** Decision acceptance is not an implementation claim. Register status, compiler behavior, tests, rulebooks, and implementation-governance status must be reconciled separately.

---

## § 2. Conversion relations and type identity

**§ 2(1)** Given:

```sec
type Name string

let number := 123
```

The following is **invalid**:

```sec
let name := Name(number)
```

The following is equally **invalid**; `try` cannot manufacture a missing conversion:

```sec
let name := try Name(number)
```

**§ 2(2)** `string(number)` remains the existing explicit, **fallible** numeric-to-text conversion, equivalent to the numeric value's zero-argument `.ToString()`; both return `Result[string, StringError]`. That primitive-`string` conversion rule is **not inherited** by every named type based on `string`.

**§ 2(3)** A named string type preserves its nominal identity and ordinary contracts. Converting an already materialized, valid `string` to `Name` uses the established explicit named-type conversion `Name(text)`, including any target-type contract checks. Neither the nominal conversion nor the contract check may be bypassed by an implicit numeric-to-string coercion.

**§ 2(4)** `Name(number)` is rejected during semantic conversion-relation validation, **before** lowering. It is not treated as `Name(string(number))`, `Name(number.ToString())`, or an infallible construction. It must not produce `Result[Name, ...]` as an alternative interpretation.

**§ 2(5)** The rule covers numeric primitives and numeric named types for which `.ToString()` is defined. It does not change ordinary conversions among the numeric types, `char`/`rune` semantics, string literal contextual typing, or explicit conversion of an already valid `string` to a named string type.

---

## § 3. Canonical `try` placement

**§ 3(1)** The **recommended** spelling is:

```sec
type Name string

fn MakeName(number: int) Result[Name, StringError] {
    let name := try Name(number.ToString())
    return Ok(name)
}
```

Here the outer `try` protects the entire following conversion expression. Under `rules/errors/errorhandling.md` § 13.1, that protected subtree includes `number.ToString()` as an argument and includes any fallible checked work of `Name(...)`.

**§ 3(2)** This is also valid for an unconstrained `Name` whose conversion from an existing `string` is infallible:

```sec
type Name string

fn MakeName(number: int) Result[Name, StringError] {
    let name := Name(try number.ToString())
    return Ok(name)
}
```

The inner `try` unwraps `Result[string, StringError]` before `Name(...)` receives the materialized `string`. This form does **not** protect a separately fallible outer `Name(...)` conversion.

**§ 3(3)** The two source forms may both be accepted when semantically well-typed, but they do not imply identical error coverage in every context. If a target named type has a runtime-checkable contract, prefer an outer `try` to cover **both** materialization and contract checking; alternatively, explicitly protect or handle each fallible step under the normal error rules.

**§ 3(4)** The already-defined general expression boundary for `try` remains authoritative. Do **not** add a conversion-specific `try` exception, new precedence rule, or implicit `Result` unwrapping outside a protected region.

**§ 3(5)** A naked `try` is legal only when every possible unhandled failure can propagate through the enclosing declared return channel. Ordinary `try` handlers may be used to handle failures locally under `errorhandling.md` §§ 11–15.

---

## § 4. Errors, contracts, and evaluation order

**§ 4(1)** Numeric `.ToString()` fails through `StringError` on its defined failure paths. That remains true when the call is nested inside `Name(...)`; no failure is silently discarded or converted into a panic.

**§ 4(2)** Conversion into a constrained named string type may fail independently through `ConversionError.Contract(...)`, following `rules/types/contracts.md`. The two error sources are distinct. The compiler must not automatically invent an anonymous union, substitute `ConversionError` for `StringError`, or infer a common public error type merely because both can occur.

**§ 4(3)** For example, this is valid where the existing common `error` channel accepts both concrete error families:

```sec
type ShortName string minLen 1 maxLen 8

fn MakeShortName(number: int) Result[ShortName, error] {
    let name := try ShortName(number.ToString())
    return Ok(name)
}
```

If text materialization fails, the conversion to `ShortName` is not attempted. If the text is produced but violates `ShortName`'s contracts, the contract failure is reported through its ordinary checked-conversion path. The first failure in source evaluation order is the only one observed on that execution path.

**§ 4(4)** An enclosing function whose declared error channel cannot accept every possible failure must handle or explicitly map the incompatible failure paths. A blanket `try` is not an automatic error-widening rule. The existing explicit `error` widening and handler rules remain authoritative.

**§ 4(5)** Contract-validity proofs already established at compile time may suppress unnecessary runtime checks under the normal contract rules. They do not grant an implicit numeric-to-named-string conversion, and they do not remove the specified fallibility of numeric text materialization.

**§ 4(6)** All normal ownership, move, borrow, allocation, cleanup, and evaluation-order rules apply. The compiler must not perform an additional, hidden numeric formatting step while carrying out `Name(text)`.

---

## § 5. Normative updates to owning rulebooks

**§ 5(1) — `rules/types/types.md`.** In "Numeric-to-text conversion (MD-051)", replace the sentence stating that conversion to a named string type is undecided (MD-093) with an explicit cross-reference to this correction and the rules in §§ 2–4. Preserve the existing `string(numericValue)` / `.ToString()` semantics. In the named-type conversion discussion, distinguish a permitted `string -> Name` explicit conversion from the forbidden direct `numeric -> Name` relation.

**§ 5(2) — `rules/library/core-library.md`.** In "ToString and formatting" and the named-type inheritance discussion, state that the primitive `string(numericValue)` convenience conversion does not automatically apply to named string types. Document the recommended explicit source sequence, `try Name(number.ToString())`. Do not add a new named-type `ToString` overload or pseudo-constructor.

**§ 5(3) — `rules/errors/errorhandling.md`.** Its existing §§ 11–14 already govern a protected call's fallible argument and the enclosing operation. A cross-reference to MD-093 may be added if useful, but **do not change** general `try` grammar or its failure-set model.

**§ 5(4) — `rules/types/contracts.md`.** Preserve the distinction between unavailable conversion relations and failures of *available* checked conversions, and preserve the defined `ConversionError.Contract(...)` channel. This correction adds no new contract form.

**§ 5(5)** Update generated documentation, compiler-known conversion documentation, LSP/hover wording, and diagnostics that currently imply direct `numeric -> named string` conversion. Do not silently change other conversion families.

---

## § 6. Compiler and IR implementation requirements

**§ 6(1)** Semantic analysis MUST resolve the source and target type identities before choosing any conversion operation. A target nominal string type must **not** be treated as primitive `string` for numeric-formatting conversion selection.

**§ 6(2)** If the source is numeric and the target is a named string type, and the only proposed relation is the primitive `string(numericValue)` formatting rule, Sema MUST reject the conversion as unavailable. This includes constant numeric values and expressions wrapped with `try`.

**§ 6(3)** For `try Name(number.ToString())`, Sema MUST retain the inner `Result[string, StringError]` failure path in the protected expression's analysis; upon success, it MUST type-check the outer `Name(string)` conversion and any declared contract checks, recording their independent fallible paths when applicable.

**§ 6(4)** Semantic IR MUST preserve the distinction between (i) explicit text materialization and (ii) explicit named-type conversion/contract validation. Neither IR construction nor lowerings may synthesize an unproven direct numeric-to-named-string operation.

**§ 6(5)** The backend MUST NOT silently produce a `Name` for an invalid direct `Name(number)` source expression. Any existing Sema acceptance of the direct conversion is a bug to be removed, not a compatibility feature.

**§ 6(6)** Existing error-handling and conversion diagnostic identities should be reused. Do not invent or assign a new stable `S####` number solely in this correction; if the registry lacks an appropriate diagnostic, update it through its established governance procedure.

---

## § 7. Required regression and conformance tests

**§ 7(1) — Rejection tests.** Given `type Name string`, verify that these are type errors for both literals and runtime numeric values:

```sec
let invalid1 := Name(123)               // Invalid: no direct conversion.
let invalid2 := try Name(123)           // Invalid: try does not add a conversion.

let number := 123
let invalid3 := Name(number)            // Invalid: no direct conversion.
let invalid4 := try Name(number)        // Invalid: try does not add a conversion.
```

Also test `int`, `uint` (including `byte`), floating, decimal, and eligible named-numeric inputs; direct conversion to a named string type must never inherit `string(numericValue)`.

**§ 7(2) — Positive tests.** Verify the two accepted forms with an unconstrained type:

```sec
type Name string

fn Outer(number: int) Result[Name, StringError] {
    return Ok(try Name(number.ToString()))
}

fn Inner(number: int) Result[Name, StringError] {
    return Ok(Name(try number.ToString()))
}
```

Verify the resulting value is a `Name` (not `string`, and not a nested `Result`) and that its text is correct. Also verify the preexisting `try string(number)` path still produces a plain `string`.

**§ 7(3) — Contract tests.** With:

```sec
type ShortName string minLen 1 maxLen 2

fn Convert(number: int) Result[ShortName, error] {
    return Ok(try ShortName(number.ToString()))
}
```

Test successful values (`1`, `12`), a violating value (`123`), and failure injection for text materialization where the test infrastructure supports controlled allocation failure. Require `ConversionError.Contract(...)` for the applicable contract failure and `StringError` for text materialization failure; never silently truncate text or turn a recoverable failure into a panic.

**§ 7(4) — `try` coverage tests.** Verify that an inner-only `try` does not authorize an unprotected fallible named-type conversion, that an outer `try` includes both fallible sites, and that an incompatible enclosing error channel yields a helpful diagnostic rather than an implicitly synthesized union/error mapping.

**§ 7(5) — IR tests.** Verify that accepted forms show a numeric-to-text operation followed by a string-to-named-type conversion/validation, while rejected direct numeric-to-named-string forms do not reach code generation. Check that source evaluation order, failure paths, and cleanup behavior are preserved.

**§ 7(6) — Stability tests.** Ensure the same rules hold through ordinary expression positions, return expressions, call arguments, and explicitly typed local initializers. Retain existing successful tests for primitive `string(number)` and `.ToString()`.

---

## § 8. Governance and closure procedure

**§ 8(1)** Integrate one feature and test group at a time: (a) update relevant paragraphs in the owning rulebooks without renumbering; (b) correct Sema and any affected conversion resolution; (c) update Semantic IR/lowering only where a genuine implementation gap exists; (d) run targeted valid/invalid and regression tests; (e) synchronize affected **existing** per-book governance fragments by stable IDs, avoiding duplicate `implemented`, `remaining`, or `identified_bugs` items.

**§ 8(2)** During implementation, record the preexisting acceptance of `Name(number)` as an identified bug if the current implementation still permits it. Do not present a newly written rule as evidence that compiler behavior is already conforming.

**§ 8(3)** Only after the owning rulebook wording and relevant governance are synchronized may **MD-093** be removed from `missing-decisions.yaml`, per the register's policy. Other open IDs, including MD-081, MD-092, MD-099, MD-100, and MD-102, are not closed by this MD-093-specific correction.

**§ 8(4)** Once integrated, archive this correction to `rules/corrections/applied/md093-named-string-conversion-correction-YYYYMMDD.md`, replacing `YYYYMMDD` with the actual application date. The archived correction records the decision; the owning rulebooks remain the canonical source for the language semantics.

---

## § 9. Acceptance record

**§ 9(1)** The owner decided that `Name(123)` MUST NOT work for a named `string` type. The programmer may use the explicitly fallible `number.ToString()` operation instead.

**§ 9(2)** The owner preferred `try Name(number.ToString())`. Inspection of the existing `try` boundaries and multi-failure rules established that the preferred spelling is already permitted and that `Name(try number.ToString())` remains legal when the outer named-type conversion does not require separate protection.

**§ 9(3)** No new syntax, error family, implicit conversion, or outstanding design decision is introduced. **MD-093 is resolved at the decision level**, pending normative synchronization and implementation verification.


## Application record — 2026-10-26

Owning types/core text, contracts and compiler-known-member cross-references
are synchronized without renumbering paragraphs. Direct numeric-to-named-string
relations are rejected before IR. Existing protected-Result machinery now
adapts conversion operands and retains their carrier failures before nominal
contract checks. Constrained named-string conversion requires its own try
protection, including fallible assignment; inner-only ToString protection
does not cover the outer conversion. The general unavailable-conversion
diagnostic was allocated through the central registry as S1174, after checking
active and retired repository references, rather than assigned by the decision.

Tests cover all numeric families and named numerics, literals/runtime values,
try/return/call/typed-local positions, both valid materialization spellings,
primitive string regression, independent core error-family/order facts,
inner-only contract protection, incompatible propagation, and CLI/LSP identity.

Runtime success text, contract-failure execution and allocation-failure injection
are not claimed verified: the accepted source's emit-ir currently stops at
unsupported intermediate-Result routing, and P23 numeric text materialization
is not implemented. Existing governance retains these backend completion gates;
the normative decision itself is closed.
