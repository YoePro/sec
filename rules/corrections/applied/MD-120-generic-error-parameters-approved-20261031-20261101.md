# MD-120 — Generic error parameters and generic error declarations

- **Decision ID:** MD-120 — `generic-error-parameters`
- **Document kind:** Approved normative correction and implementation handoff
- **Status:** **APPROVED — D1–D7**; applied to the canonical rulebooks and governance 2026-11-01; implementation verification pending
- **Created / approved:** 2026-10-31
- **Document revision:** 1.0
- **Sec language version:** **0.1**
- **Reviewed repository:** `YoePro/sec`, `main` at `1eba0fd61642f2273a2ff9918ab453006c72bc80` (verified 2026-10-31)
- **Supersedes:** `rules/corrections/applied/md120-generic-error-parameters-proposal-20261031-superseded.md` (proposal revision 0.1)
- **Normative owners:** `rules/declarations/generics.md`, `rules/errors/errorhandling.md`, `rules/types/types.md`, `rules/declarations/interfaces.md`, `rules/declarations/unions.md`, `rules/declarations/enums.md`
- **Integration owners:** `rules/compiler/monomorphization.md`, `rules/compiler/semantic_ir.md`, `rules/tooling/diagnostics.md`, `rules/tooling/lsp.md`, applicable governance entries and focused conformance tests
- **Related:** MD-029 (error patterns), MD-057 (borrowed interface views), MD-117 (heterogeneous type packs), MD-118 (`fmt` public API)

> **Integration instruction.** The language owner approved D1–D7 individually on 2026-10-31. These decisions are authoritative for the Sec 0.1 specification, **not** evidence that parser, Sema, Semantic IR, lowering, LSP or library implementations are complete. Synchronize one numbered rulebook section or implementation step at a time, test each change, preserve existing section numbers and do not create duplicate governance owners. Archive this correction as applied only once normative integration is complete; implementation completion has separate evidence gates.

---

## § 1. Purpose and boundary

**§ 1(1).** A generic parameter may express a compile-time requirement that its actual type belongs to Sec's existing error-type category. Such a parameter may be used as the error channel of `Result[T, E]`, preserving an exact channel through generic interfaces and functions.

**§ 1(2).** This is an extension of existing generic constraint resolution. It does **not** make `error` an interface, introduce structural constraints, dynamic exception handling, implicit erasure, general subtyping or a new runtime generic dispatch mechanism.

**§ 1(3).** Generic template bodies are checked under their declared requirements before specialization, as in `monomorphization.md` § 5.1. Concrete specialization must not retroactively legitimize a source expression that the template's declared constraints do not justify.

## § 2. D1 — Syntax and permitted constraint categories (approved)

**§ 2(1).** The exact spelling is `[E: error]`. Here `error` is a **compiler-known error-category requirement** in the normal constraint position after `:`. It is neither an interface type nor a requirement for explicit `implements error` conformance.

```sec
interface FormatSink[E: error] {
    mut fn WriteText(text: string) Result[void, E]
}
```

**§ 2(2).** Generic constraints in Sec 0.1 are existing interface requirements or the compiler-known category `error`. They may be combined using the existing `&` grammar, for example `[E: error & Printable]`; every conjunct must independently be satisfied through its own rules. The category requirement supplies no arbitrary methods.

**§ 2(3).** `[T: int]` remains **invalid**: `int` is an ordinary concrete type, neither a generic constraint interface nor an approved category requirement. This decision creates no generic numeric-family or conversion-based constraint. Parameter names such as `E`, `ErrorType`, `T` or `E1` have no special meaning.

**§ 2(4).** `[E error]`, reserving the identifier `E`, and implicit inference of an error requirement from use of `Result[T, E]` are not admitted.

## § 3. D2 — Admitted type arguments (approved)

**§ 3(1).** An actual type argument satisfies `E: error` precisely if it satisfies the existing error-type eligibility of `Result[Success, Argument]`: the compiler-known root `error` or a declared Sec error type, including error-marked enum/union types and other already-defined applicable error types. This decision does not broaden which declarations may be marked `error`.

```sec
FormatSink[IOError]  // A specific, closed error channel.
FormatSink[error]    // The open error root as the explicit argument.
```

**§ 3(2).** `FormatSink[string]` is invalid: `string` does not satisfy the error category. Diagnostics must identify the argument, parameter and failed requirement.

**§ 3(3).** The two concrete generic interface instantiations above are **distinct**. The normal widening `IOError -> error` for individual error values does **not** imply `FormatSink[IOError] -> FormatSink[error]` conversion, conformance, or interface-view substitution. No implicit generic variance is added.

## § 4. D3 — Template operations, error propagation and ownership (approved)

**§ 4(1).** An `E: error` parameter may appear in `Result[T, E]`, `Option[E]`, `ref E`, `ref mut E` when ordinarily lawful, generic payloads, and other type positions that admit a type argument. It has the error-category guarantee and any additional explicitly declared interface guarantees—nothing more.

**§ 4(2).** With the expected channel `Result[T, E]`, `Err(value)` accepts a value assignable to `E`, and `Ok(value)` / `Ok()` follow the normal `Result` success rules. Construction of a particular `E` variant is unavailable unless the template has a separately valid, explicitly named concrete type or capability.

**§ 4(3).** Naked `try` may propagate `Result[U, E]` through an enclosing compatible `Result[T, E]` function. It may also propagate a value of `E` into an enclosing `Result[T, error]` through the existing error-specific widening, preserving actual error identity and payload. This does **not** create arbitrary cross-channel conversion, `Result` covariance, or hidden wrapper errors.

```sec
fn Forward[E: error](sink: ref mut FormatSink[E], text: string) Result[void, E] {
    try sink.WriteText(text)
    return Ok()
}
```

**§ 4(4).** `Err(e)` binds the whole error value of type `E`; catch-all matching and handling retain the normal ownership, guard and scope rules. `Err(_)` is permitted **only when discarding the matched payload is legal and provable** under the ordinary discardability rules (`errorhandling.md` § 17). Merely requiring `E: error` does not establish discardability.

**§ 4(5).** Template code cannot use a variant pattern of unknown `E`—including `Err(E.SomeVariant)` or contextual `Err(.SomeVariant)` when the only expected channel is generic `E`. It cannot assume members, construct an error variant, narrow `E` to a particular concrete error type, or compare `E` values absent an applicable additional constraint. By contrast, the **known outer variants** of an explicitly declared generic error union may be matched even when a variant's payload has type `E` (§ 5).

**§ 4(6).** `E: error` does **not** imply `Copy`, discardability, defaultability, or a trivial destructor. Owned/move-only payloads and explicit consuming call-site moves obey the existing ownership rules; returning a value follows the ordinary return-boundary ownership rule. Instantiations must preserve these obligations and must not add implicit copying.

**§ 4(7).** A match on `Result[T, E]` may exhaustively cover `Ok(...)` and a whole-channel `Err(...)` arm, with normal payload-discard rules. The compiler may not infer the variants of `E` from possible concrete instantiations and then accept an otherwise invalid template match.

## § 5. D4 — Generic error type declarations (approved)

**§ 5(1).** Existing generic enum and generic union declarations may carry the existing `error` marker. A fully substituted instance is a distinct nominal error type, eligible as the error argument of `Result`. No new declaration form or marker position is introduced.

```sec
type ForwardError[E: error] union error {
    Format(FormatError)
    Sink(E)
}

enum OperationError[T] error {
    Failed
    Cancelled
}
```

**§ 5(2).** The enclosing declaration's `error` marker does **not** require every one of its generic type arguments to be an error type. An ordinary payload parameter may be any permitted type:

```sec
type ParseError[T] union error {
    InvalidValue(T)
    UnexpectedEnd
}
```

**§ 5(3).** `ForwardError[IOError]` and `ForwardError[error]` are distinct types. Each has statically known outer variants `Format` and `Sink`; the latter variant holds the exact selected `E`. An outer `Sink(payload)` match is valid in a generic body, but a variant test *inside* payload `E` is not justified without further type knowledge.

**§ 5(4).** The enum remains payload-less; a phantom type parameter affects nominal identity but creates no stored member. Generic enum underlying representation, defaultability and specialization follow the existing enum rulebook.

**§ 5(5).** This decision does **not** introduce a new way to declare error-marked generic named aliases/types beyond forms already specified by their owning rulebooks. No new inheritance, implicit mapping between error families, or auto-wrapper is added.

## § 6. D5 — Generic interfaces, borrowed views and conformance (approved)

**§ 6(1).** An implementation must name the complete concrete interface specialization. On the implementing side, the existing interface rule requires an ordinary `fn` body to satisfy a `mut fn` contract; this decision does not authorize `mut fn` on the implementing method.

```sec
type StdoutSink struct {}

impl StdoutSink implements FormatSink[IOError] {
    fn WriteText(text: string) Result[void, IOError] {
        return Ok()
    }
}
```

**§ 6(2).** A borrowed interface view names its instantiated error type, e.g. `ref mut FormatSink[IOError]` or `ref mut FormatSink[E]` in a generic body. A bare `FormatSink` / `ref mut FormatSink` without the required argument is invalid. There is no implicit default argument `error` and no automatic erasure to it.

**§ 6(3).** Generic functions may accept `ref mut FormatSink[E]` with ordinary MD-057 borrowed-view dispatch. `E` is fixed at compile time for the specialization; the concrete implementing object may still be dynamically dispatched through the instantiated interface view. No owned interface value, boxing or heap allocation is implied.

**§ 6(4).** Where an argument's explicit valid interface conformance determines **exactly one** `FormatSink[E]` instantiation compatible with the call, ordinary generic inference may derive `E` from that conformance. If more than one legal compatible candidate remains, inference must not guess; explicit generic arguments or other unambiguous context must resolve it. This is **not** permission to create otherwise illegal duplicate or conflicting implementations.

**§ 6(5).** A conformance to `FormatSink[IOError]` does not automatically satisfy or produce a view of `FormatSink[error]`. Error-value widening alone cannot change the instantiated interface's identity (§ 3(3)).

## § 7. D6 — Inference and explicit generic arguments (approved)

**§ 7(1).** `E: error` uses the existing inference mechanisms from ordinary arguments, receiver/owner, expected result type when that context supplies a unique answer, explicit type arguments, and the unambiguous interface-conformance case in § 6(4).

```sec
fn WrapError[E: error](cause: E) Result[void, E] {
    return Err(cause)
}

fn Ready[E: error]() Result[int, E] {
    return Ok(10)
}

let first := WrapError(IOError.Closed)
let second: Result[int, IOError] := Ready()
let third := Ready[IOError]()
```

**§ 7(2).** A result-only `E` without sufficient expected-type context remains uninferrable: `let result := Ready()` is invalid. There is **no** default/fallback to `error`, even though `error` itself is an allowed explicit argument. An explicit `Result[int, error]` expected type may infer `E = error` normally; that does not constitute a fallback.

**§ 7(3).** The existing rules for explicit arguments, positional partial argument lists, conflict resolution, diagnostics and generic specialization apply unchanged. Error-type constraints are checked at the requesting instantiation and are not inferred structurally from names or methods.

## § 8. D7 — Version and activation (approved)

**§ 8(1).** All of §§ 2–7 are part of **Sec language version 0.1**. There is no 0.1.1 gate for `[E: error]`, generic error enums/unions, or the compatible generic interface surface. This is a normative target, not evidence of deployed compiler support.

**§ 8(2).** The owner's version boundary is that **0.1.1 starts the progression toward 0.2**. MD-120 therefore must not be deferred solely to avoid adding it to the almost-complete 0.1 specification.

## § 9. Interaction with core formatting and MD-118

**§ 9(1).** Existing `sec/core/format.sec` declares `interface FormatSink[E]` with `Result[void, E]`, but currently uses uninstantiated `FormatSink` parameters in `_WriteFormatSegment` and `FormatComposite`; `sec/stdlib/fmt/print.linux.amd64.sec` likewise declares `_StdoutFormatSink implements FormatSink` without the argument. After approval these are integration defects: the interface requirement must be `[E: error]` and uses must name or infer the complete type. Keep the existing method capability syntax (`mut fn` in the interface; `fn` in the implementation).

**§ 9(2).** The current `FormatRenderError` and stdlib `WriteFormatError` are concrete and include formatter/text errors plus `IOError`. Converting a destination from `IOError` to generic `E` must **not silently lose** format/`StringError` failures or output-size overflow failures, nor misclassify an independent formatter failure as a sink failure. The final destination-independent composite-format API, exact generic wrapper error variants and any refactoring of `FormatRenderError` belong to **MD-118** and its canonical `fmt` specification. MD-120 decides only that generic error-channel types and wrappers are legal, not their final public `fmt` names or signatures.

**§ 9(3).** Where a generic sink result is propagated, keep the existing error-channel and ownership checks. No compiler-special `fmt` overload, implicit `any` conversion, or generic erased error object is authorized.

## § 10. Required rulebook reconciliation — preserve existing section identifiers

**§ 10(1).** `rules/declarations/generics.md`: extend §§ 11–14 to allow the compiler-known error-category constraint alongside interface constraints; add a distinct, stable subsection after § 15 documenting the guarantees in § 4; update §§ 5–6, 18–20, 22–23 and 31 to show error-marked generics, precise inference and prohibited `[T: int]`. Amend examples of compiler-known `Result` to show its requirement, without suggesting that user code may redeclare the predeclared type.

**§ 10(2).** `rules/errors/errorhandling.md`: extend §§ 2–4 and 12 to admit `E: error`, preserve `error`-specific widening and unknown variant handling; explicitly cross-reference § 17 discardability and § 29 ownership. Add an appropriately numbered subsection for generic error enum/union declarations, without renumbering existing §§ 3.1–3.4.

**§ 10(3).** `rules/types/types.md`: update "Core generic result and option types" to allow a generic parameter constrained by the error category in `Result[T, E]`. Preserve the existing prohibition on arbitrary non-error types and the distinct meaning of the error root.

**§ 10(4).** `rules/declarations/interfaces.md` §§ 7.1 and 9: clarify fully instantiated generic borrowed interface views, error-constrained generic interface declarations, matching explicit conformances and unique conformance-derived generic inference; retain existing MD-057 view/dispatch constraints and `mut fn` implementation rules.

**§ 10(5).** `rules/declarations/unions.md` § 16 and `rules/declarations/enums.md` §§ 3 and 16.1: admit the existing `error` marker on their generic declarations and preserve the separate concrete nominal identity and payload/representation rules.

**§ 10(6).** `rules/compiler/monomorphization.md`: reflect pre-specialization guarantees and per-instantiation error identity/ownership; `rules/compiler/semantic_ir.md`: preserve the exact instantiated error channel rather than treating generic `E` as implicitly open `error` or inventing an unverified representation.

**§ 10(7).** `rules/tooling/diagnostics.md` and `rules/tooling/lsp.md`: reuse existing constraint failure IDs when applicable, allocate new IDs only for genuinely distinct diagnostic cases, and ensure hover/completion and source locations distinguish constrained unknown `E`, the explicit root `error`, and a concrete instantiation. Do not claim source support before implementation.

**§ 10(8).** Update `missing-decisions.yaml`: once the owning rulebooks and applicable governance contracts are synchronized, change MD-120 from `open` to the appropriate decided/implementation-pending state. Remove it only under the registry's existing closure policy. Use existing owner entries in `governance/generics.yaml`, `governance/errors.yaml`, `governance/declarations.yaml`, `governance/types.yaml` and the corresponding compiler/tooling entries; add targeted remaining tasks there instead of duplicating owners. `governance/stdlib.yaml` owns MD-118 API completion separately.

## § 11. Minimum conformance and regression matrix

**§ 11(1). Positive parser/Sema cases:** `[E: error]`, `[E: error & Printable]` with a valid conformer, `Result[void, E]` in an interface and generic function, concrete error enum/union arguments, explicit `FormatSink[error]`, `FormatSink[IOError]`, generic `union error` with `Sink(E)`, a generic `enum error` with phantom `T`, an error union with non-error payload `T`, and every normally permitted error-root widening.

**§ 11(2). Negative parser/Sema cases:** `[E error]`, `[T: int]`, unconstrained `Result[T, E]`, `FormatSink[string]`, an omitted interface argument, invented implicit interface variance, unauthorized `E` variant patterns, arbitrary `E` member access, copying or discarding an `E` where no such permission is proved, and incompatible concrete error-channel propagation.

**§ 11(3). Inference cases:** infer `E` from ordinary argument, expected result type and one unambiguous explicit conformer; reject `Ready()` without context, incompatible combined constraints, and genuinely ambiguous conformances; test explicit and valid partial generic argument forms. Reject hidden fallback to `error`.

**§ 11(4). Ownership/pattern cases:** `Err(e)` binding for possibly move-only `E`, `Err(_)` with discardable versus non-discardable payloads, exact propagation and widening without clones, match of known outer variants of `ForwardError[E]`, and rejection of generic-inner variant selection.

**§ 11(5). Backend cases:** distinct source/semantic specialization identities for `E = IOError` and `E = error`; exact error payload and destruction facts; no implicit interface erasure or conversion; no partial emitted module on an unsupported lowering boundary. Tests requiring unimplemented ownership, generic interface view or physical `error` representation should remain explicitly gated rather than be reported as passing.

**§ 11(6). Tooling/stdlib cases:** LSP hover/signature help and targeted diagnostics for the new constraint; corrected `FormatSink` declarations and complete uses; an explicit MD-118 handoff preserving formatter, materialization, output-count and destination failures.

## § 12. Completion criteria

**§ 12(1).** The language design is **decided** (D1–D7). No additional owner design vote is required merely to implement an already implied consequence. New unresolved semantics must be recorded separately and must not silently change MD-120.

**§ 12(2).** Normative closure requires synchronized rulebooks and tracking with stable references. Implementation closure requires the relevant parser/Sema/monomorphization/IR/tooling test evidence; failures that belong to other packages or MD-118 remain explicitly assigned there. Do not conflate these milestones.
