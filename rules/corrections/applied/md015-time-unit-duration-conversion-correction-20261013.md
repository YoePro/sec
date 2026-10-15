# MD-015 — Time Quantity Literals and Conversion to `duration`

- **Status:** Applied 2026-10-13 to `rules/types/temporal.md` (§ 5, revision 1.2), `rules/types/units.md` ("Quantity literals", revision 2.1), `rules/foundations/grammar.md` ("Quantity literals"), `rules/types/types.md` (temporal types), `rules/library/core-library.md` (§ 10.3 `RoundingMode`) and the timeout paragraphs of `mutex.md` (§§ 12(1), 23(8), 57(11)), `channels.md` (§ 28(7)), `select.md` (§ 30(3)) and `cancellation.md` (§ 43(3)); core declares `s`/`ms`/`us`/`ns` once with `Kind: time` and `RoundingMode`; the frontend implements quantity literals and the conversion (S1153), while Semantic IR, lowering and duration arithmetic remain tracked by `frontend.temporal-duration`. MD-040 and MD-047 remain open; the `impl <Unit>` spelling of § 3.3 is not yet the catalog spelling. **Amended 2026-10-15** by `rules/corrections/applied/md015-md061-duration-reborrow-correction-20261015.md`: § 6.3 (contextual conversion of range-proven runtime typed quantities) is superseded — only quantity literals are materialized contextually — and the matching § 10.2 test is rewritten.
- **Created:** 2026-10-13
- **Last updated:** 2026-10-13
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/corrections/applied/md015-time-unit-duration-conversion-correction-20261013.md`
- **Decision ID:** `MD-015`
- **Primary rulebooks:** `rules/types/units.md`, the canonical temporal rulebook, `rules/foundations/grammar.md`
- **Affected rulebooks:** `rules/types/types.md`, `rules/library/core-library.md`, `rules/concurrency/mutex.md`, `rules/concurrency/channels.md`, `rules/concurrency/select.md`, `rules/concurrency/cancellation.md`
- **Related decisions:** `MD-012` (`ConversionError`), `MD-027` (contextual argument typing), `MD-040` (`<Unit>` name resolution), `MD-047` (`RoundingMode` and explicit rounding)
- **Replaces:** No existing numbered paragraph. Amend established paragraphs in place; do not renumber their public identifiers.
- **Governance:** Synchronize existing `frontend.temporal-duration` and `frontend.units-v2` entries and their dependencies; do not add duplicate status records. Remove `MD-015` from the latest authoritative `missing-decisions.yaml` only after all owning normative rules and governance have been synchronized.

---

## 1. Purpose and scope

**1.1.** This correction defines the previously missing source and semantic contract for numeric quantity expressions such as `5<s>` and `250<ms>` and their conversion to Sec's compiler-known/core `duration` type.

**1.2.** A quantity carrying the physical dimension of time is **not** automatically a `duration`. Conversion to `duration` is permitted where expressly requested or where an expected `duration` type provides context, subject to the exactness and representability rules below. Without such context, the expression remains a unit-bearing numeric quantity and participates in ordinary unit algebra.

**1.3.** A canonical conversion must be defined once in the temporal/unit model. Mutex, Channel, Select, Task, Context, and similar APIs must not invent their own time-unit parsing, unit-name tables, rounding, or timeout overloads.

**1.4.** This correction does not change the identity or general APIs of `duration`, define calendar durations, decide general logarithmic/affine conversions (`MD-048`), or introduce any new keyword.

## 2. Exact quantity-expression syntax

**2.1.** The canonical tightly-bound literal form is a numeric literal followed by an ordinary unit annotation:

```text
TimeQuantityLiteral
    ::= (IntegerLiteral | FloatingLiteral) UnitAnnotation

UnitAnnotation
    ::= "<" UnitExpression ">"
```

`UnitAnnotation` and `UnitExpression` retain the existing grammar and compiler-known unit name resolution. `TimeQuantityLiteral` is a semantic specialization of the general unit-bearing numeric literal; the frontend must specify that shared production rather than creating a time-only parser. A preceding unary sign belongs to the unary-expression grammar, not to the literal token.

**2.2.** Exact accepted source examples:

```sec
let seconds := 5<s>
let milliseconds := 250<ms>
let microseconds := 10<us>
let nanoseconds := 3<ns>
let negative := -5<s>
let fractional := 0.5<ns>
```

**2.3.** The `<...>` part is resolved through normal unit symbols and dimensions, not through a fixed list in the parser or individual APIs. The same grammar must work for ordinary non-temporal quantities:

```sec
let distance := 100<m>
let elapsed := 5<s>
let velocity := distance / elapsed
```

`elapsed` does **not** change to `duration` in this expression. The derived dimension is length per time, as determined by normal unit algebra.

**2.4.** Where no expected carrier type exists, the numeric carrier is determined by the established general unit-literal and unit-default-carrier rules. This correction does not force every time quantity to `duration`, nor does it invent an integer carrier solely because the source literal is integral.

**2.5.** Lexing/parsing must distinguish `5<s>` as one quantity-expression form from ordinary comparisons, generic/array syntax, and type annotations. Invalid unit names or malformed unit expressions receive ordinary compile-time unit/grammar diagnostics. Formatter output must preserve canonical adjacent spelling such as `500<ms>`.

## 3. Time units available in `core`

**3.1.** The fundamental time units `s`, `ms`, `us`, and `ns` belong to the compiler-known/core unit environment and are available without importing the standard library. This is a unit-availability decision, not a requirement for a distinct temporal wrapper type for each spelling.

**3.2.** Their canonical relationships, relative to one second, are exact:

| Unit | Dimension | Scale relative to `s` |
|---|---|---:|
| `<s>` | time | 1 |
| `<ms>` | time | 1/1,000 |
| `<us>` | time | 1/1,000,000 |
| `<ns>` | time | 1/1,000,000,000 |

All four denote linear **time differences/interval quantities**, not absolute calendar or monotonic-clock points.

**3.3.** The complete minimal source declarations for these four core units, using the separately approved `MD-040` canonical `impl <Unit>` syntax, are:

```sec
module core

unit s physical

impl <s> {
    LongName: "Second"
    Symbol: "s"
    Kind: time
    Dimension: [time^1]
    Scale: 1
    System: SI
}

unit ms physical

impl <ms> {
    LongName: "Millisecond"
    Symbol: "ms"
    Kind: time
    Dimension: [time^1]
    Scale: 1 / 1000
    System: SI
}

unit us physical

impl <us> {
    LongName: "Microsecond"
    Symbol: "us"
    Kind: time
    Dimension: [time^1]
    Scale: 1 / 1000000
    System: SI
}

unit ns physical

impl <ns> {
    LongName: "Nanosecond"
    Symbol: "ns"
    Kind: time
    Dimension: [time^1]
    Scale: 1 / 1000000000
    System: SI
}
```

These declarations must exist **once**, under the canonical compiler-known/core authority. If an existing core or platform unit catalog already owns a definition, amend/migrate it instead of publishing a second conflicting unit. Existing optional metadata defaults remain governed by `rules/types/units.md`; the spelling here does not invent additional metadata fields.

**3.4.** Neither the compiler nor concurrency APIs may infer that any unit with a convenient short name is a time quantity. The canonical resolved dimension, transform, kind, and exact scale facts must be used. Other valid user-defined time units can participate through the same generic rules; they do not require named special cases.

## 4. `duration` representation and domain

**4.1.** `duration` is the existing compiler-known/core time-interval type. Its canonical storage/value model for Sec 0.1 is a **signed 64-bit integer count of nanoseconds**, with range:

```text
-9223372036854775808 .. 9223372036854775807
```

There is no requirement to introduce a separate uppercase `Duration` type or an ordinary user-visible constructor declaration solely to implement conversions.

**4.2.** Negative, positive and zero durations are valid values. Negative duration is essential for time differences and is not inherently an error:

```sec
let before := duration(-5<s>)
let none := duration(0<ns>)
let after := duration(5<s>)
```

**4.3.** API-specific constraints are separate. An operation such as `Wait(d: duration)` may impose a non-negative timeout contract, but this must not change the domain of `duration` or silently clamp its argument. Such a contract is owned by the individual API's normative rulebook.

## 5. Exact conversion to nanoseconds

**5.1.** For a time-dimension numeric quantity `q`, compute its exact mathematical duration in seconds using the resolved **linear** unit transform, then multiply by `1,000,000,000` to obtain the exact mathematical nanosecond count:

```text
exactNanoseconds = exactQuantityInSeconds * 1000000000
```

For the core time units, this reduces to the exact relationships of § 3.2. The conversion does not pass through a target-dependent `float32`/`float64` approximation or round an intermediate value.

**5.2.** Without an explicitly chosen rounding mode, a conversion succeeds only when the exact result is an **integer** nanosecond count within the signed `int64` range. No implicit truncation, saturation, wraparound, hidden precision loss, or sign reversal is allowed.

**5.3.** Positive and negative quantities obey exactly the same precision and overflow rules.

**5.4.** An incompatible dimension, an unsupported point/difference role, an unresolved or runtime-supplied scale, or a transform not admitted by the ordinary time-difference rules must not be silently accepted as `duration`. These remain ordinary unit/conversion compatibility failures; `MD-048` separately owns specialized affine/logarithmic conversion rules.

## 6. Explicit and contextual source forms

**6.1.** Explicit conversion, expected-type declaration initialization, and call argument contextual conversion are all valid when the conversion is provably exact and representable:

```sec
let explicitValue := duration(5<s>)
let declaredValue: duration := 500<ms>

fn Wait(d: duration) void {
}

Wait(500<ms>)
```

**6.2.** A bare time quantity is never automatically a `duration` merely because the source unit has dimension time:

```sec
let elapsed := 5<s>                  // Unit-bearing numeric quantity.
let converted := duration(elapsed)   // Explicitly requests duration.
```

**6.3.** Contextual conversion is allowed only when the compiler can prove the conversion valid for the actual expression, or for **all values of a statically known bounded source domain**. For example, a runtime value of an appropriately bounded integer millisecond type may be safely converted if its entire possible range fits in `int64` nanoseconds and the scale is integral in nanoseconds.

**6.4.** An arbitrary dynamic numeric time quantity for which exactness or `int64` range cannot be proved must **not** be silently converted at a `duration` call boundary or assignment. The programmer must use an explicit checked conversion and handle the failure according to normal Sec `try`/`Result` rules:

```sec
fn CheckedTimeout(source: decimal<ms>) Result[duration, ConversionError] {
    let timeout := try duration(source)
    return Ok(timeout)
}
```

This example illustrates the checked path for a value whose domain cannot be proved valid, not a claim that every `decimal<ms>` instance fails conversion.

**6.5.** Overload selection continues to depend on input parameters, not return types (the previously approved `MD-027` rule). Contextual typing of a time quantity must not cause ambiguous overloads to be resolved by guessing or by inspecting the call's expected result type.

## 7. Explicit rounding (`MD-047` dependency)

**7.1.** If the exact mathematical nanosecond count is fractional, the default conversion is invalid. A programmer may explicitly provide the previously approved `RoundingMode` as an additional conversion argument:

```sec
let a := duration(500<ms>)
let b := duration(0.5<ns>, RoundingMode.Ceil)
let c := duration(-0.5<ns>, RoundingMode.Floor)
```

Here `a` is 500,000,000 ns, `b` is +1 ns, and `c` is −1 ns.

**7.2.** The accepted enumeration, owned once by numeric core (not by `stdlib/math`), is:

```sec
enum RoundingMode {
    NearestEven
    NearestAwayFromZero
    TowardZero
    AwayFromZero
    Ceil
    Floor
}
```

This repeats the *already approved* `MD-047` signature for cross-rulebook exactness; it does **not** authorize a second independent declaration. The ordinary `math.Round()` API belongs to `stdlib/math` and is separate from conversion lowering.

**7.3.** Rounding, when explicitly requested, occurs **exactly once after** the full mathematical time-unit conversion and before the `int64` range check. The chosen policy is applied to the mathematical nanosecond count, not to the original quantity or to an intermediate numeric carrier. All modes follow their mathematical definitions, including for negative values.

**7.4.** The following is rejected as a known inexact conversion; supplying an explicit rounding policy is required:

```sec
let invalid := duration(0.5<ns>)  // ERROR: cannot represent half a nanosecond.
```

**7.5.** Supplying a rounding mode does not permit overflow, incompatible units, undefined scales, or another unrelated conversion error. No implicit rounding mode is selected.

## 8. Errors, overflow and checked arithmetic

**8.1.** A **statically provable invalid** constant conversion (inexact without a rounding mode, invalid unit relationship, or out-of-range result) is a compile-time error under the existing compile-time conversion rules.

**8.2.** An explicit dynamic checked conversion that can fail uses the already approved core `ConversionError`, including `ConversionError.OutOfRange` for a final nanosecond count outside signed `int64` and `ConversionError.PrecisionLoss` for a default exact conversion that produces a fractional nanosecond count. Its call is protected by `try` or otherwise handled using the established error-handling syntax.

**8.3.** Ordinary arithmetic on existing `duration` values obeys Sec's established checked-overflow rules. No duration-specific wrapping, clamping, saturation, or silent loss is introduced:

```sec
let span := duration(5<s>)
let offset := duration(-2<s>)
let combined := span + offset   // 3 seconds; ordinary checked duration arithmetic.
```

**8.4.** The compiler must not introduce an overflowing intermediate multiplication when the mathematical conversion is representable or falsely accept an overflowing final result. A backend may use a provably correct widened/intermediate representation or equivalent checked arithmetic; the source semantics remain the exact mathematical conversion and final signed 64-bit range check.

**8.5.** A target/profile that cannot implement the defined checks and result semantics must reject unsupported operations at compile time instead of silently changing temporal meaning. This does not itself establish a new allocation requirement.

## 9. Concurrency and target integration

**9.1.** All concurrency and context APIs keep their existing canonical `duration` parameter declarations; this correction introduces **no** unit-specific overloads:

```sec
fn Wait(d: duration) void {
}

Wait(250<ms>)
```

**9.2.** `Mutex.Lock(250<ms>)`, `Context.WithTimeout(5<s>)`, Channel/Select timeout forms, and any other declared `duration` boundary use the same conversion plans and diagnostics. Names such as `<ms>` are resolved by the selected canonical unit environment, never by per-API hard-coded spellings.

**9.3.** Negative input policy, zero-timeout behavior, cancellation, timeout commit ordering, and target runtime timebase are owned by each API/temporal rulebook. Nothing in this correction silently declares every negative timeout valid, and nothing globally forbids negative `duration` values.

## 10. Required conformance tests

**10.1. Quantity and type preservation:**

1. Parse, type and format `5<s>`, `250<ms>`, `10<us>`, `3<ns>`, `-5<s>`, `0.5<ns>`.
2. Verify `let elapsed := 5<s>` remains unit-bearing and does not infer `duration`.
3. Verify `100<m> / 5<s>` participates in length/time dimensional algebra and does not materialize its time operand as `duration`.
4. Verify unit names are resolved from core/unit metadata; no API-specific list of timeout unit spellings is used.

**10.2. Exact conversion:**

1. `duration(5<s>)` yields 5,000,000,000 ns.
2. `duration(500<ms>)` yields 500,000,000 ns.
3. `duration(1<us>)` yields 1,000 ns and `duration(1<ns>)` yields 1 ns.
4. `duration(-5<s>)` yields −5,000,000,000 ns and `duration(0<ns>)` yields zero.
5. A typed `duration` declaration and a call taking `duration` accept a provably exact time quantity without an invented overload.
6. A dynamic quantity whose whole possible domain is proved exact and in range is accepted; one lacking such proof is not implicitly accepted.

**10.3. Rounding and precision:**

1. Reject `duration(0.5<ns>)` at compile time.
2. Check `duration(0.5<ns>, RoundingMode.Ceil)` yields +1 ns.
3. Check `duration(-0.5<ns>, RoundingMode.Floor)` yields −1 ns.
4. Check all six rounding modes on positive, negative, integral and halfway cases.
5. Verify no intermediate rounding changes the final nanosecond count.

**10.4. Range and diagnostics:**

1. Verify exact lower and upper signed `int64` nanosecond boundaries succeed.
2. Reject constants one nanosecond beyond either boundary.
3. Verify dynamic inexact and overflowing checked conversions return `ConversionError.PrecisionLoss` and `ConversionError.OutOfRange` respectively.
4. Verify ordinary `duration` arithmetic never silently wraps or saturates.
5. Reject incompatible dimensions, invalid time-unit metadata and unresolved transforms with precise compiler diagnostics.
6. Verify the target/profile never changes these language-level outcomes.

**10.5. Call boundaries and regression:**

1. Verify mutex, channel, select and context duration call sites all resolve through one conversion implementation.
2. Verify normal overload resolution is retained and return types do not resolve ambiguity.
3. Verify Semantic IR records the exact source unit, scale, proof or checked conversion, rounding policy when present, and final signed duration value/representation.
4. Verify cold/incremental compilation and formatter/LSP diagnostics agree with the canonical rulebook.

## 11. Integration instructions and closure

**11.1.** Integrate §§ 2–5 and 7–8 into the canonical temporal/unit rulebooks and their referenced grammar, using the published paragraph numbering without renumbering. The repository's current public tree may not expose a separate `rules/types/temporal.md`; resolve the **actual current temporal owner** from the authoritative repository before writing an integration path. Do not invent a new competing temporal authority.

**11.2.** Ensure the ordinary core `duration` implementation and the selected core time-unit catalog realize one identity each. Update documentation/hover and compiler-known declaration registries to reflect exact source forms; no placeholder public signatures or invented constructors are allowed.

**11.3.** Update every owning concurrency timeout paragraph to reference the shared conversion model instead of a locally described or hard-coded list of units. Preserve existing API signature, cancellation and commit semantics.

**11.4.** Apply the previously approved `MD-047` rounding-mode semantics consistently; do not create a new temporal-only enum or duplicate `math.Round` in core. Integrate the `MD-040` `<Unit>` syntax before treating § 3.3 examples as compiler-accepted.

**11.5.** Update existing governance fragments for temporal/unit conversions and run the focused conformance tests. **Only after integration, verification and synchronization** may the latest authoritative missing-decisions document remove `MD-015`. This correction does not by itself claim `MD-047`, `MD-048`, or any separate issue is fully integrated or closed.
