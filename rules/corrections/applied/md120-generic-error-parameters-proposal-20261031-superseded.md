# MD-120 — Generic error parameters (`[E: error]`)

- **Status:** Superseded by the owner-approved `MD-120-generic-error-parameters-approved-20261031-20261101.md`; kept as background only
- **Decision ID:** MD-120
- **Created:** 2026-10-31
- **Document revision:** 0.1
- **Sec language version:** 0.1
- **Destination after approval:** `rules/corrections/applied/md120-generic-error-parameters-correction-YYYYMMDD.md`
- **Primary normative owners:** `rules/declarations/generics.md` §§ 2, 11–15, 22–23, 31; `rules/errors/errorhandling.md` §§ 2–4, 12; `rules/types/types.md` "Core generic result and option types"
- **Related owners:** `rules/declarations/interfaces.md` (generic interfaces and borrowed views, MD-057); `rules/declarations/unions.md` and `rules/declarations/enums.md` (error markers); `rules/compiler/monomorphization.md`; `rules/compiler/semantic_ir.md`; `rules/tooling/diagnostics.md`; `rules/tooling/lsp.md`
- **Related decisions:** MD-008/MD-029 (error-variant patterns), MD-057 (interface views), MD-117 (type packs), MD-118 (fmt public API)
- **Trigger:** `sec/core/format.sec` declares `interface FormatSink[E] { mut fn WriteText(text: string) Result[void, E] }`, which no current rule admits.

---

## § 1. Problem

**§ 1(1).** `Result[T, E]` requires `E` to be the compiler-known `error` root or a declared concrete error type (`types.md` "Core generic result and option types"; `errorhandling.md` § 4). A generic template is validated before specialization from its declared contracts alone (`monomorphization.md` § 5.1). An unconstrained parameter `E` is not known to be an error type, so `Result[void, E]` cannot be valid in a template.

**§ 1(2).** Constraints are interface requirements (`generics.md` §§ 11, 13). `error` is a compiler-known root type, not an interface. `[E: error]` is therefore currently invalid.

**§ 1(3).** The only expressible alternative today is the open root: `Result[void, error]`. It loses the concrete error type statically: callers must narrow, and the sink's error channel can no longer be matched exhaustively.

**§ 1(4).** The same need appears wherever an abstraction forwards a caller-chosen error channel: sinks and writers, readers, parsers over a source, retry and transaction wrappers, iterators over fallible sources, and the `fmt` surface of MD-118.

## § 2. D1 — Spelling of the requirement

| Option | Form | Assessment |
|---|---|---|
| **A (recommended)** | `[E: error]` | Reuses the existing constraint position. `error` is a compiler-known *category constraint*, alongside interface constraints. Combines with `&`: `[E: error & Printable]`. The predeclared `Result` itself becomes expressible as `type Result[T, E: error] union`. |
| B | `[E error]` | Mirrors the declaration marker `enum IOError error { … }`. It is a second, marker-style constraint syntax, so `[E error & Printable]` has no natural form, and it differs from every other generic requirement. |
| C | Reserve the name `E` | Name-based semantics. Rejected: a parameter's name would change its meaning, so two error parameters (`[E1, E2]`) or a non-error `E` would be impossible. It conflicts with `generics.md` § 2 (parameter names are ordinary identifiers) and with the rule against automatic structural constraints (§ 31). Diagnostics and LSP would have to explain hidden meaning. |
| D | No generic errors | Keep `Result[T, error]`. The status quo; § 1(3) describes its cost. |

**Recommendation:** A. Reserving `E` (option C) should not be chosen: it is the only option that makes an identifier's spelling semantic.

## § 3. D2 — Admitted arguments

**§ 3(1).** Proposed: a concrete argument for `E: error` is any type for which `Result[T, Arg]` is valid today, which is the `error` root or a declared concrete error type (enum error, union error, error-marked named type).

**§ 3(2).** Sub-question D2a: may the argument be the `error` root itself (`FormatSink[error]`)? **Recommendation: yes.** It is the ordinary open channel, and an adapter for heterogeneous sinks needs it. Each specialization still has one concrete error channel type.

**§ 3(3).** A non-error type (`FormatSink[string]`) is a constraint failure at the instantiation, with the ordinary constraint diagnostic (§ 9).

## § 4. D3 — Operations available on `E` inside a template

Proposed template environment for `E: error` (`generics.md` § 15):

1. `Result[T, E]`, `Option[E]`, `ref E` and other type formations that accept an error type.
2. `Err(value)` where `value: E`, and `Ok(...)` into `Result[T, E]`.
3. Propagation: `try` on a `Result[U, E]` operand inside a function returning `Result[T, E]` (same channel), and widening `E → error`. Because every admitted argument widens to `error`, `try` on `Result[U, E]` inside a function returning `Result[T, error]` is valid.
4. Catch-all handling: `Err(e)`, `Err(_)`, and `try … { Err(e) => … }` bind the whole value of type `E`.
5. Not available: variant patterns (`Err(.Variant)`, `Err(E.Variant)`), member access, construction of an `E` value, `E → ConcreteError` narrowing and comparison, because no particular variant set or operation is known. Any further capability comes only from an added interface constraint (`[E: error & Describable]`).
6. Ownership: `E` is not assumed copyable (`generics.md` § 16; `monomorphization.md` § 11.3). A template moves an `E` value explicitly (`<-error`), exactly as for any other unconstrained parameter. D3a: should `E: error` imply copyability? **Recommendation: no**, because union errors may own data (`errorhandling.md` § 3.3).
7. Exhaustiveness: a match on `Result[T, E]` is exhaustive with `Ok(...)` and a catch-all `Err(...)` arm, which is the same as for the open root.

## § 5. D4 — Generic error declarations

**§ 5(1).** Combining errors needs generic error *types*, for example:

```sec
type FormatWriteError[E: error] union error {
    Format(FormatError)
    Sink(E)
}
```

**§ 5(2).** Proposed: generic unions, enums and named types may carry the `error` marker. Their instances are concrete error types, so `FormatWriteError[IOError]` is a closed union error with two variants. A specialization preserves the payload's concrete identity, and widening the outer instance to `error` follows `errorhandling.md` § 2.1.

**§ 5(3).** D4a: is a generic error declaration admitted in Sec 0.1 (**recommended: yes**, since without it D1 cannot express combined errors), or deferred to 0.1.1 like other generic library types?

## § 6. D5 — Interfaces, implementations and borrowed views

**§ 6(1).** A generic interface `interface FormatSink[E: error]` is implemented per concrete argument:

```sec
impl Stdout implements FormatSink[IOError] {
    fn WriteText(text: string) Result[void, IOError] { … }
}
```

**§ 6(2).** A borrowed interface view must name its argument: `ref mut FormatSink[IOError]`. A bare `ref mut FormatSink`, as written in `format.sec` today, has no error channel type. Proposed: it is an error that asks for the argument, and no default or erasure to `error` happens.

**§ 6(3).** A generic algorithm over any sink takes the channel as a parameter:

```sec
fn WriteAll[E: error](sink: ref mut FormatSink[E], parts: string[]) Result[void, E] {
    for part in parts {
        try sink.WriteText(part)
    }
    return Ok()
}
```

D5a: confirm that the error argument of a borrowed generic interface view is inferred from the argument's concrete conformance, as for any other generic argument (`generics.md` § 18), with no special rule.

## § 7. D6 — Inference and explicit arguments

Proposed: no special rule. `E` is inferred from arguments and expected types like any other parameter (§ 18), and explicit and partial explicit arguments follow §§ 19–20. An `E` that only occurs in the return type and cannot be inferred is the ordinary "cannot infer" error. A context that propagates into `Result[T, error]` does not choose `E = error` implicitly.

## § 8. D7 — Activation version

The decision adds source capability. D7: does it ship in Sec 0.1 (needed for the current `format.sec` design and MD-118), or 0.1.1? **Recommendation: 0.1**, because MD-118's public `fmt` sink API depends on it and the change is additive.

## § 9. Diagnostics (proposed)

1. `E` used as a Result error type without `: error` — a template error naming the parameter, with help `declare it as [E: error]`.
2. Constraint failure — `T=string does not satisfy error` at the instantiation, with the template parameter as related location.
3. Variant pattern on `E` — no variants are known for the error parameter `E`; match the whole value with `Err(e)` or add an interface constraint.
4. Bare generic interface view — `FormatSink` requires its error argument, `FormatSink[ErrorType]`.
5. Option C is not adopted, so no diagnostic depends on a parameter's spelling.

## § 10. Prepared rulebook amendments (apply only after approval of A)

**§ 10(1) `generics.md` § 11 (append):** "A generic parameter may be constrained by the compiler-known category `error`: `[E: error]` requires the concrete argument to be the `error` root or a declared error type (`rules/errors/errorhandling.md` §§ 2–4). It is not an interface constraint and grants only the operations of § 15A. It combines with interface constraints by `&`."

**§ 10(2) `generics.md` § 13 (amend first sentence):** "Each constraint must resolve to an interface type, a valid concrete generic interface instantiation, or the compiler-known category `error`."

**§ 10(3) `generics.md` new § 15A "Error parameters":** § 4 of this proposal, items 1–7.

**§ 10(4) `generics.md` § 23 (amend example):** `type Result[T, E: error] union { Ok(T) Err(E) }`, with a sentence that the predeclared `Result` carries this requirement.

**§ 10(5) `errorhandling.md` § 4 (append):** "`E` may also be a generic parameter constrained by `error` (`generics.md` § 15A). Within the template it behaves as an error channel whose variants are unknown; each specialization has the concrete error type of its argument."

**§ 10(6) `errorhandling.md` § 3 (new § 3.5 "Generic error types"):** § 5(2) of this proposal.

**§ 10(7) `types.md` "Core generic result and option types" (amend):** "`E` must be the compiler-known `error` root, a concrete Sec type declared as an error type, or a generic parameter constrained by `error`."

**§ 10(8) `interfaces.md` (generic interfaces and views):** § 6(1)–(2) of this proposal.

**§ 10(9) `diagnostics.md`:** allocate identities for § 9 items 1, 3 and 4. Item 2 reuses the constraint-failure identity.

**§ 10(10) `lsp.md`:** hover on `E` shows `E: error` and, in a specialization, its concrete argument. Completion after `Err(` in a template offers only catch-all bindings.

## § 11. Conformance tests (after approval)

- Positive: `interface FormatSink[E: error]`; `impl X implements FormatSink[IOError]`; `fn WriteAll[E: error](sink: ref mut FormatSink[E], …) Result[void, E]` with `try` propagation; specializations for an enum error, a union error and the `error` root; widening `try` into `Result[T, error]`; `type W[E: error] union error { Sink(E) … }` matched exhaustively in a concrete instance.
- Negative: `Result[T, E]` with unconstrained `E`; `FormatSink[string]`; `Err(.Variant)` on `E`; copying an `E` without `<-`; a bare `ref mut FormatSink`; `E → IOError` narrowing.
- Specialization: distinct Semantic IR specializations per error argument (`monomorphization.md` § 9.1); with the current IR ownership gate, owned-data errors stay a named boundary until per-specialization ownership plans exist.

## § 12. Interaction notes

- **MD-117:** `fn FormatComposite[E: error, ...Types](sink: ref mut FormatSink[E], …)` combines an error parameter with a type pack; no further rule is needed.
- **MD-118:** the public `fmt` result contract can then be `Result[uint, FormatWriteError[E]]` for a caller's sink instead of `error`.
- **Implementation:** Sema needs the category constraint, template checks of § 4 and the § 9 diagnostics. Semantic IR needs no new concept, because a specialization substitutes a concrete error type.
