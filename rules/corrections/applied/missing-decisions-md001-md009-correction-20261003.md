# Missing Decisions MD-001 through MD-009 — Correction

**Status:** Applied correction  
**Applied:** 2026-10-03  
**Created:** 2026-10-03  
**Updated:** 2026-10-03  
**Revision:** 1  
**Sec version:** 0.1  
**Canonical path:** `rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md`  
**Replaces:** `rules/corrections/applied/missing-decisions-md001-md004-correction-20261002.md`
**Partially superseded by:** `rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md` § 3 (contract argument grammar: ordinary `Expression` in a `SemanticCompileTimeRequiredContext` replaces `ConstantExpression`, including `RegexContract`)

## 1. Purpose and scope

1.1. This correction records the accepted Sec 0.1 decisions for `MD-001` through `MD-009` from `missing-decisions.yaml`.

1.2. The affected owning rulebooks and governance fragments must be synchronized with this correction.

1.3. An entry must not be removed from `missing-decisions.yaml` merely because this correction exists. Remove an entry only after all owning rulebooks record the decision and affected governance has been synchronized.

1.4. This correction does not decide `MD-010` or any later missing decision except where a paragraph below explicitly states that a question remains owned by a later decision.

1.5. Where this correction describes a migration diagnostic, sloppy correction, bad-practice diagnostic, or recovery behavior, that behavior is part of the Sec 0.1 tooling contract and must not silently change language semantics.

---

## 2. MD-001 — Confusable identifiers

2.1. Sec uses Unicode Technical Standard #39 (UTS #39) confusable detection for identifier-confusability checks.

2.2. The normative confusable mapping data is the Unicode `confusables.txt` data corresponding to the same Unicode version used by the compiler's other Unicode tables.

2.3. Compiler updates must not independently update confusable data to a different Unicode version from the compiler's other Unicode data.

2.4. Identifier identity remains the exact NFC-normalized identifier spelling. A UTS #39 skeleton is used only for confusability detection and never changes identifier identity.

2.5. A confusable-identifier check is performed only where two declarations can actually conflict in the same name-resolution domain.

2.6. Two visually confusable identifiers in unrelated scopes or unrelated functions are legal.

2.7. Example:

```sec
fn First() {
    let admin := 1
}

fn Second() {
    let аdmin := 2
}
```

The two identifiers above may be confusable, but they are legal because the declarations are in unrelated function scopes.

2.8. A confusable collision in one conflicting declaration domain is a compile-time error, not a warning.

2.9. The confusable-collision diagnostic is not suppressible.

2.10. The diagnostic must identify the conflicting declarations and explain that the identifiers are distinct spellings whose UTS #39 confusable forms collide in the same declaration domain.

2.11. MD-001 does not define the namespace behavior of unit symbols. Unit-symbol namespace behavior is defined by MD-007 in § 8 of this correction.

---

## 3. MD-002 — Named type declaration syntax

3.1. The canonical Sec 0.1 named-type declaration form is:

```text
type Name TypeReference
```

3.2. A named-type declaration has exactly one underlying `TypeReference`.

3.3. Examples:

```sec
type A int
type B string
type C float
```

3.4. A Sec named type is distinct from its underlying type. It is not a transparent alias.

3.5. Therefore:

```sec
type B int
type A B
```

declares three distinct types: `int`, `B`, and `A`.

3.6. The following is not valid Sec 0.1 syntax:

```sec
type A int string
```

3.7. The following alias-like spelling is not valid Sec 0.1 syntax:

```sec
type A = int
```

3.8. The following compact multi-variant spelling is not valid Sec 0.1 syntax:

```sec
type A = First Second
```

3.9. Sum or variant types must use the explicit Sec enum or union declaration syntax owned by their respective rulebooks.

3.10. When an obsolete `=` named-type spelling is recognized unambiguously, the compiler should issue a focused migration diagnostic rather than accepting it as compatibility syntax.

3.11. Sloppy correction may rewrite a recognized obsolete named-type spelling only when the rewrite is unambiguous and semantics-preserving.

---

## 4. MD-003 — Legacy syntax, migration diagnostics, and sloppy correction

4.1. Sec 0.1 accepts canonical Sec 0.1 syntax. Obsolete spellings are not accepted as an alternative compatibility grammar.

4.2. When the compiler can recognize an actual legacy Sec spelling unambiguously, it should issue a focused migration diagnostic explaining the canonical Sec 0.1 spelling.

4.3. The ordinary formatter must not silently make invalid legacy source valid.

4.4. Explicit sloppy correction may rewrite invalid legacy syntax only when the rewrite is unambiguous and semantics-preserving.

4.5. If a migration is ambiguous, sloppy correction must not guess. The compiler must issue a focused diagnostic instead.

4.6. Sec 0.1 aggregate declarations retain the current aggregate declaration syntax, including forms such as:

```sec
type User struct {
}
```

4.7. A future Sec version may use forms such as:

```sec
struct User {
}
```

That future syntax is not canonical Sec 0.1 syntax and must not be treated as accepted Sec 0.1 syntax.

4.8. The prefix sequence spelling `[]byte` was never normative Sec syntax and must not be documented as a legacy Sec form.

4.9. The canonical sequence spelling is:

```sec
byte[]
```

4.10. Owning rulebooks may enumerate additional safe migration rewrites, but every such rewrite remains subject to §§ 4.3–4.5.

---

## 5. MD-004 — Runtime string concatenation and interpolation

5.1. Runtime string concatenation and runtime string interpolation are fallible materialization operations.

5.2. A concatenation or interpolation expression is semantically one materialization operation producing one resulting `string`.

5.3. The language does not require observable intermediate strings.

5.4. The compiler may measure all parts, prepare final storage once, write all parts into that storage, and produce the final string when observable Sec semantics are preserved.

5.5. Runtime materialization failure uses Sec's ordinary fallible-operation mechanism.

5.6. A runtime string materialization that may allocate therefore requires ordinary `try` handling.

5.7. Example:

```sec
let message := try prefix + name + suffix
```

5.8. A concatenation or interpolation fully resolved at compile time may be folded into static materialized data and is not a runtime fallible operation.

5.9. Optimization may remove allocations or temporaries only when success behavior, failure behavior, ownership, lifetime, destruction, and observable value semantics remain unchanged.

5.10. Runtime string materialization uses the canonical active allocation context defined by `rules/memory/allocation.md`.

5.11. Sec 0.1 defines no string-specific allocator and no string-specific allocation domain.

5.12. The frontend resolves the applicable allocation context before Semantic IR.

5.13. Semantic IR preserves the resolved allocation domain or context together with the ordinary failure channel, ownership facts, and lifetime facts required by lowering.

5.14. Backend lowering must consume those resolved facts and must not choose an allocator independently.

5.15. Allocation failure from runtime string materialization uses the canonical `AllocationError` type defined by the allocation rules.

5.16. When the resulting string escapes a local allocation lifetime, materialization must use an allocation context whose lifetime is sufficient for the result.

5.17. A backend must not repair an invalid allocation lifetime by silently reallocating the result into another allocation domain.

5.18. If no valid allocation context is available, compilation fails.

5.19. On a target that prohibits dynamic allocation, runtime string materialization is valid only when the compiler proves that the dynamic allocation is eliminated while preserving Sec semantics.

---

## 6. MD-005 — Unescaped closing brace in interpolated string text

6.1. In interpolated-string text, a literal closing brace is written as `}}`.

6.2. An unmatched single `}` in interpolated-string text is invalid source.

6.3. The mandatory diagnostic identity is:

```text
L1021
lexer.unescaped-interpolation-closing-brace
```

6.4. The primary diagnostic range is exactly the offending single `}`.

6.5. The diagnostic must explain both the error and the repair. The diagnostic wording must communicate that a single `}` is not valid in interpolated-string text and that `}}` must be written for a literal closing brace.

6.6. The malformed interpolated-string candidate must be retained for recovery rather than replacing the whole candidate with one generic `ILLEGAL` token.

6.7. A frontend that represents an interpolated string as one token retains that token as malformed and preserves its exact source spelling.

6.8. A frontend that represents interpolation as segmented tokens retains the offending `}` in the surrounding malformed text segment.

6.9. Recovery treats the offending `}` as malformed source text only and remains in interpolated-string text mode.

6.10. Recovery after the offending brace must continue to recognize later valid interpolation openings and the eventual closing quote.

6.11. Existing unterminated-string newline and EOF boundaries remain the recovery boundary when no valid closing quote is reached.

6.12. The recovery representation must not make the source valid and must not produce a successfully compilable string value as though `}}` had been written.

6.13. Each independently encountered unmatched single `}` reached during valid recovery produces its own `L1021` diagnostic.

---

## 7. MD-006 — `is` state-test expressions

7.1. A non-binding `is` state test is an ordinary Sec expression of type `bool`.

7.2. A non-binding `is` state test may appear in every expression context where a `bool` expression is valid.

7.3. Examples:

```sec
let idle := state is Idle
let missing := option is None

return state is Running

if ready && state is Idle {
    Use()
}
```

7.4. `is` belongs to the equality/state-test precedence level.

7.5. Equality and state-test operators at that level are non-chainable.

7.6. Therefore:

```sec
ready && state is Idle
```

groups as:

```sec
ready && (state is Idle)
```

7.7. A state-test result may be compared explicitly after parentheses, but redundant comparison with `true` is bad practice.

7.8. Example:

```sec
(state is Idle) == true
```

is semantically valid but must produce a bad-practice diagnostic recommending:

```sec
state is Idle
```

7.9. Explicit sloppy correction may rewrite the redundant `== true` form in § 7.8 to the canonical direct state test.

7.10. `is not` is the canonical direct negation of a non-binding state test.

7.11. Examples:

```sec
state is not Idle
state is not empty
option is not None
place is not available
```

7.12. The exact set of state designators valid for a type is defined by the rulebook that owns that type or state operation.

7.13. The ordinary boolean form:

```sec
!(state is Idle)
```

is semantically valid.

7.14. Because Sec has the direct canonical form:

```sec
state is not Idle
```

the form in § 7.13 must produce a bad-practice diagnostic recommending the direct `is not` spelling.

7.15. Explicit sloppy correction may rewrite:

```sec
!(state is Idle)
```

to:

```sec
state is not Idle
```

when the rewrite is unambiguous and semantics-preserving.

7.16. Ordinary prefix precedence still applies. Sec does not give `!` a special parsing rule merely because an `is` expression follows.

7.17. The positive Option binding form remains a narrow Sec 0.1 condition facility:

```sec
if option is Some(value) {
    Use(value)
}
```

7.18. The binding in `Some(value)` exists only on the positive path on which the `Some` variant matched.

7.19. MD-006 does not generalize `is Variant(binding)` into an arbitrary pattern expression.

7.20. In Sec 0.1, the positive `Option` binding form is not valid as a general stored or returned boolean expression, and it is not generalized to arbitrary union or Result variants.

7.21. The following is invalid:

```sec
if option is not Some(value) {
}
```

7.22. The form in § 7.21 cannot bind `value` because the true path states that the value is not the `Some` variant and therefore has no `Some` payload to bind.

7.23. The compiler must issue a focused diagnostic that explains this reason and should recommend either:

```sec
if option is not Some {
}
```

when no payload is required, or a positive `Some(value)` condition when the payload is required.

7.24. The following is invalid for the same binding reason:

```sec
!(option is Some(value))
```

7.25. Sloppy correction must not repair `is not Some(value)` or `!(option is Some(value))` by deleting the binding, because deleting the binding would discard explicit programmer intent.

7.26. Sec 0.1 does not introduce binding scope through short-circuit `&&` or `||` as part of MD-006.

---

## 8. MD-007 — Unit-symbol namespace

8.1. Unit symbols live in a separate unit-symbol namespace from ordinary declarations and ordinary identifiers.

8.2. Unit-symbol lookup occurs only in unit syntax and unit-expression contexts such as `<...>`.

8.3. An ordinary identifier does not conflict with a unit symbol merely because the two have the same spelling.

8.4. Example:

```sec
unit s

fn Example() {
    let mut s: int := 10
}
```

is valid.

8.5. In particular, ordinary identifier `s` and unit symbol `<s>` are distinct names in distinct namespaces.

8.6. The same rule applies to unit symbols such as `ns`.

8.7. Duplicate and ambiguity checks still apply within the unit-symbol namespace itself.

8.8. Ordinary no-shadowing checks must not reject an ordinary local solely because its spelling is also used by a unit symbol.

8.9. The current S1057 exemption for unit names is therefore conceptually correct and must be aligned with the explicit namespace rule in this section.

8.10. Confusable-identifier rules from MD-001 operate within the relevant name-resolution domain and must not manufacture a conflict between an ordinary identifier and a unit symbol solely across these separate namespaces.

---

## 9. MD-008 — Concrete error variants in `match`

9.1. This section defines a deliberately narrow Sec 0.1 rule.

9.2. In Sec 0.1, an ordinary `match` on `Result[T, ConcreteError]` may use:

```sec
Err(ConcreteError.Variant)
```

to match one specific variant of the concrete error payload.

9.3. Example:

```sec
match result {
    Ok(value) => Use(value)

    Err(IOError.NotFound) => HandleMissing()
    Err(IOError.AccessDenied) => HandleDenied()
    Err(IOError.Timeout) => HandleTimeout()
}
```

9.4. For a closed concrete error type, specific `Err(ConcreteError.Variant)` arms participate in ordinary closed exhaustiveness checking.

9.5. If all possible concrete error variants are covered together with the required `Ok` coverage, the Result match may be exhaustive without a general `Err(error)` arm.

9.6. If one or more concrete error variants remain uncovered, the match is non-exhaustive unless another valid arm covers them.

9.7. `Err(error)` binds the complete error payload and covers the complete `Err` branch.

9.8. Therefore:

```sec
match result {
    Ok(value) => Use(value)
    Err(error) => Handle(error)
    Err(IOError.NotFound) => HandleMissing()
}
```

contains an unreachable final arm.

9.9. This ordering is valid:

```sec
match result {
    Ok(value) => Use(value)
    Err(IOError.NotFound) => HandleMissing()
    Err(error) => Handle(error)
}
```

9.10. The existing open-error narrowing behavior for `Result[T, error]` is unchanged by this correction.

9.11. MD-008 does not introduce general recursive or nested pattern matching in Sec 0.1.

9.12. `Err(ConcreteError.Variant)` is an explicit Sec 0.1 Result/error refinement exception to the otherwise restricted nested-pattern model.

9.13. This Sec 0.1 exception is intentionally narrow.

9.14. A later Sec version may replace, generalize, or otherwise change this exception if Sec adopts a more general nested or destructuring pattern model.

9.15. Implementations and documentation must therefore not present the narrow Sec 0.1 exception as a permanent general pattern-model guarantee.

---

## 10. MD-009 — `regex` contract grammar and declaration-name reservation

10.1. In Sec 0.1, `regex` is a compiler-known reserved contract spelling.

10.2. `regex` is unavailable as a user declaration name.

10.3. Examples of invalid user declarations include:

```sec
let regex := "abc"

fn regex() void {
}

type regex string
```

10.4. Reservation as a contract spelling does not require `regex` to have a dedicated hard-keyword token kind.

10.5. `regex` may be lexed identifier-like and resolved contextually in contract position, consistently with other contextual compiler-known contract spellings.

10.6. The canonical grammar includes a `RegexContract` alternative under `TypeContract`.

10.7. The canonical production is:

```text
RegexContract
    ::= Contextual("regex") ConstantExpression
```

10.8. A canonical use has the shape:

```sec
type Email string regex "..."
```

10.9. Semantic analysis requires the `ConstantExpression` used by a regex contract to produce the compile-time string pattern required by `rules/types/contracts.md`.

10.10. MD-009 does not decide the regular-expression dialect, escape semantics after string decoding, anchoring semantics, matching unit, complexity guarantees, backreference policy, or runtime regex engine. Those questions remain owned by MD-010.

10.11. MD-009 does not decide the complete compile-time-expression classification or which named compile-time values and calls are permitted in contract positions. Those questions remain owned by MD-011.

10.12. There is no special physical same-line requirement for the `regex` contract.

10.13. Both of the following are grammatically permitted before canonical formatting is applied:

```sec
type Email string regex "..."
```

```sec
type Email string
    regex "..."
```

10.14. Newline handling for this grammar follows Sec's ordinary whitespace and expression-continuation rules. A parser must not invent a hidden rule requiring `regex` to occur on the same physical line as the preceding type or contract.

10.15. The formatter owns the canonical presentation of a valid regex contract.

10.16. `regex` must be added to the compiler-known reserved contract-word inventory in `rules/foundations/lexical_structure.md`.

10.17. The existing normative length-contract spellings `minLen`, `maxLen`, and `exactLen` have the same rulebook-synchronization problem if they are absent from the grammar or reserved contract-word inventory.

10.18. Synchronizing `minLen`, `maxLen`, and `exactLen` with their already normative contracts is a specification synchronization fix, not a new language-design decision.

---

## 11. Required rulebook synchronization

11.1. MD-001 requires synchronization of at least:

- `rules/foundations/lexical_structure.md`
- the relevant diagnostics/tooling rules
- affected governance fragments

11.2. MD-002 requires synchronization of at least:

- `rules/foundations/grammar.md`
- affected named-type/type rules
- affected governance fragments

11.3. MD-003 requires synchronization of at least:

- `rules/foundations/grammar.md`
- `rules/declarations/struct.md`
- `rules/collections/collections.md`
- `rules/types/contracts.md`
- formatter/migration rules where applicable
- affected governance fragments

11.4. MD-004 requires synchronization of at least:

- `rules/foundations/operators.md` where string concatenation semantics are owned
- `rules/foundations/lexical_structure.md` where interpolation semantics are owned
- `rules/compiler/semantic_ir.md`
- `rules/memory/allocation.md`
- affected governance fragments

11.5. MD-005 requires synchronization of at least:

- `rules/foundations/lexical_structure.md`
- `rules/tooling/diagnostics.md`
- affected lexer governance fragments

11.6. MD-006 requires synchronization of at least:

- `rules/foundations/operators.md`
- `rules/declarations/unions.md`
- `rules/control-flow/flowcontrol_if.md`
- `rules/control-flow/flowcontrol_while.md`
- `rules/memory/ownership.md`
- formatter/sloppy-correction rules
- affected governance fragments

11.7. MD-007 requires synchronization of at least:

- `rules/foundations/names_scopes_visibility.md`
- `rules/types/units.md`
- affected name-resolution governance fragments

11.8. MD-008 requires synchronization of at least:

- `rules/errors/errorhandling.md`
- `rules/control-flow/flowcontrol_match.md`
- `rules/corrections/applied/match-errorhandling-correction-20260824.md` or a later superseding correction
- affected parser/sema governance fragments

11.9. MD-009 requires synchronization of at least:

- `rules/types/contracts.md`
- `rules/foundations/grammar.md`
- `rules/foundations/lexical_structure.md`
- affected parser/type-contract governance fragments

11.10. After each decision has been incorporated into all owning rulebooks and affected governance fragments, the corresponding `MD-001` through `MD-009` entry may be removed from `missing-decisions.yaml`.

11.11. The synchronization work must preserve existing paragraph identifiers or paragraph numbers where other rulebooks, diagnostics, tests, governance fragments, or source comments reference them.

---

## 12. Implementation discipline

12.1. Compiler work applying this correction must not invent semantics beyond the decisions stated here.

12.2. Where this correction explicitly leaves a question to MD-010, MD-011, or another later decision, implementation must preserve the unresolved state rather than silently selecting behavior.

12.3. Diagnostics introduced or changed by this correction must follow Sec's diagnostic principle: explain what is wrong, why it matters where useful, and how the programmer can resolve it.

12.4. Frontend recovery behavior must not make invalid source semantically valid.

12.5. Lower compiler stages must consume semantic decisions already resolved by the frontend and Semantic IR and must not reinterpret the source-language rules defined by this correction.
