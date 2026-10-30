# MD-029, MD-042 and MD-102 — Error Patterns, Proven Variant Reachability and Contextual `Some`

| Field | Value |
|---|---|
| **Status** | **APPLIED 2026-10-30** — normatively integrated 2026-10-28 (grammar.md, errorhandling.md §§ 15.3, 20, 27.2b, flowcontrol_match.md §§ 5, 25A, unions.md, types.md, functions.md § 20, generics.md § 18.1, semantic_ir.md § 71(6), diagnostics.md § 24(4e)-(4g), formatter.md §§ 20(11)-(12), lsp.md). MD-042 and MD-102 are implemented through Semantic IR facts; MD-029 parser, Sema, ownership, diagnostics, formatter, LSP and the § 17 conformance cases are implemented. MD-029 Semantic IR/Sec MLIR lowering of error-pattern plans (Semantic IR rejects with unsupported-lowering today) and the Sec 0.1.2 activation gate of Err(.Variant...) remain in governance/errors.yaml frontend.errorhandling-v2. |
| **Created** | 2026-10-28 |
| **Last updated** | 2026-10-28 |
| **Document revision** | 1.0 |
| **Language baseline** | Sec 0.1; additive contextual `.Variant` notation targeted for **Sec 0.1.2 or 0.1.3**, at the owner's choice |
| **Repository** | `YoePro/sec`, `main` |
| **Verified repository HEAD** | `7be87775a851be60a692a50010d1ab368550b7d1` (2026-10-28) |
| **Decision IDs** | `MD-029`, `MD-042`, `MD-102` |
| **Suggested correction destination** | `rules/corrections/applied/md029-md042-md102-pattern-reachability-some-correction-20261028.md` |
| **Primary normative owners** | `rules/foundations/grammar.md`; `rules/errors/errorhandling.md`; `rules/control-flow/flowcontrol_match.md`; `rules/declarations/unions.md`; `rules/types/types.md`; `rules/declarations/functions.md`; `rules/tooling/diagnostics.md` |
| **Related normative owners** | `rules/declarations/generics.md`; `rules/memory/ownership.md`; `rules/memory/borrowing.md`; `rules/memory/copy_move.md`; `rules/memory/destruction.md`; `rules/compiler/semantic_ir.md`; `rules/tooling/lsp.md`; `rules/tooling/formatter.md`; applicable Sec MLIR dialect and lowering rules |
| **Existing implementation owners** | `frontend.errorhandling-v2` (`governance/errors.yaml`); `frontend.match-v2` (`governance/control_flow.yaml`); `frontend.types-core` (`governance/types.yaml`); relevant existing parser, Sema, generic-inference, IR, lowering and diagnostics entries |
| **Related prior decisions** | MD-008 (specific `Err` variant matching); MD-027 (contextual `None`/empty literals); MD-028 (try failure sets); MD-057 and ordinary ownership rules where applicable |
| **Supersedes** | Only incompatible or incomplete provisions concerning MD-029, MD-042 and MD-102, **not** their other existing semantics |

> **MANDATORY IMPLEMENTATION DISCIPLINE FOR CLAUDE/CODEX:** Read the current `main` HEAD and compare its repository sync identifier before accessing or modifying repository files. Reuse unchanged source material. Implement **one numbered phase at a time**, run its focused tests, inspect and correct failures, and only then advance. Do not batch implementation phases and run tests at the end. Preserve existing normative paragraph numbers, stable diagnostic IDs and governance entry IDs. This document records language decisions; its existence does not establish that any phase is implemented.

---

## § 1. Authority, scope and invariants

**§ 1(1).** The Sec owner approved all three design decisions on 2026-10-28. This document gives the normative correction to apply to the canonical rulebooks and dependent compiler/tooling contracts. Each affected canonical rulebook must be amended in place, retaining stable existing paragraph and section identities; newly needed clauses are additive.

**§ 1(2).** MD-029 extends the **existing narrow `Result` error-pattern exception** to payload-carrying concrete union errors, shallow field extraction, qualified narrowing of open error channels and contextually abbreviated variant names. It does **not** enable arbitrary recursive patterns, runtime reflection or a new general pattern language.

**§ 1(3).** MD-042 specifies how provably restricted subject states affect `match` exhaustiveness, legal superfluous arms and advisory diagnostics. It does not relax type checking, ownership safety or genuine duplicate/covered-arm errors.

**§ 1(4).** MD-102 gives `Some(value)` contextual and, when the payload is independently typeable, self-inferred `Option[T]` construction. Ordinary overload resolution, generic inference, lifetime checks, copy/move restrictions and explicit transfer syntax remain authoritative.

**§ 1(5).** No parser shape, backend encoding or optimization is a substitute for these source semantics. Unknown or unproved facts are not treated as static knowledge. A compiler implementation must not claim conformance by silently treating a rejected feature as a different valid program.

**§ 1(6).** The general `case .Variant`, ordinary enum construction `.Variant`, arbitrary `Some(Ok(...))` destructuring, and broadly generalized inferred namespace proposals in `Ideas for version 0.2.md` remain outside this correction. Only the explicitly specified `Err(.Variant...)` shorthand is advanced to the 0.1.2/0.1.3 target. No general Sec 0.1 shorthand is implied.

---

## § 2. MD-029: precise existing error-carrier model

**§ 2(1).** The relevant existing carriers are `Result[T, E]` with a **closed concrete** enum/union `E`, and `Result[T, error]` with an **open** error channel. The concrete error identity and payload are preserved when a concrete error widens to the compiler-known root `error`.

**§ 2(2).** Both `try expression { Err(...) => ... }` and `match result { Ok(...) => ...; Err(...) => ... }` use the **same error-pattern selection and binding semantics**. `try` handlers are intentionally partial, so unmatched failures propagate according to existing `try` rules. `match` remains exhaustive over its proven reachable states.

**§ 2(3).** Existing forms stay legal and unchanged:

```sec
Err(_)
Err(errorValue)
Err(ArithmeticError.DivisionByZero)
Err(FileError.PermissionDenied)
```

`Err(_)` is an explicit error discard and remains subject to the existing discardability check. A whole-error binding follows the existing error-channel and ownership rules. Neither form is reinterpreted as a new general nested pattern.

**§ 2(4).** `Err(errorValue)` binds the **whole error**, not the payload of one error variant. `Err(FileError.NotFound(info))` selects one concrete variant and binds **that variant's payload**. These are different bindings with different refined types.

---

## § 3. MD-029: payload-bearing error variants

**§ 3(1).** A concrete union-error variant carrying a single payload, including a struct-like payload, may be selected in an `Err` pattern and have its payload bound as one value:

```sec
// Illustrative public error declaration.
type FileError union error {
    NotFound {
        Path: string
    }

    PermissionDenied

    InvalidEncoding(uint)
}

// Within a try handler or Result match arm:
Err(FileError.NotFound(info)) => Log(info.Path)
Err(FileError.InvalidEncoding(position)) => ReportPosition(position)
```

**§ 3(2).** The same concrete union-error variant supports **shallow named-field destructuring** using the existing union field-pattern grammar:

```sec
Err(FileError.NotFound { Path: path }) => Log(path)
```

Fields may be omitted, named under a new binding name, explicitly borrowed, or explicitly mutably borrowed only when the receiving value and ownership context grant the corresponding authority. The existing field-pattern rules apply without weakening.

**§ 3(3).** Destructuring is exactly **one level** deep. The field pattern binds a field; it does not recursively inspect a union, struct or `Result` stored in that field. `Some(Ok(value))` and an arbitrary nested `Err`/union pattern are not added to Sec 0.1. Use a second `match` to inspect a nested value.

**§ 3(4).** The variant name and structural payload shape must be resolved against the **actual declared variant** of the concrete error type. An unknown variant, unknown field, duplicate field, wrong payload shape, illegal binding mode or incompatible payload type remains a compile-time error, with a focused diagnostic.

**§ 3(5).** A single-payload variant supports the existing whole-payload discard spelling `Variant(_)` where otherwise legal. A struct-like payload supports explicit empty field destructuring `Variant { }`. These forms make deliberate non-binding visible in source and are not implicit transfers.

---

## § 4. MD-029: variant-only patterns and payload-awareness information

**§ 4(1).** Selecting a union variant is distinct from extracting its payload. The **bare variant pattern** is valid even for a variant with a payload, both in ordinary union `match` and in the special error patterns:

```sec
// Valid even when FileError.NotFound carries Path.
Err(FileError.NotFound) => GetDefaultContent()

// Ordinary union match uses the same rule.
match figure {
    Figure.Rectangle => HandleRectangle()
    Figure.Circle => HandleCircle()
}
```

These examples do not independently establish that an entire `match` is exhaustive; all reachable variants still need coverage.

**§ 4(2).** A bare variant selects every value of that variant, irrespective of payload contents. It is **not** a test that the payload is empty or default. An unguarded bare variant contributes full coverage of that variant; a guarded bare variant contributes only the guarded subset under existing coverage rules.

**§ 4(3).** When a bare variant pattern selects a declaration with payload but binds none of it, the compiler should issue a **configurable Information diagnostic** by default. This is guidance, **not** a compilation failure and **not** a warning by default. The diagnostic explains that data exists, identifies the payload type or fields, and suggests a legal binding form.

```text
Information: FileError.NotFound carries a payload that this pattern does not inspect.

Available field:
  Path: string

To inspect it:
  Err(.NotFound { Path: path })

To explicitly ignore its fields:
  Err(.NotFound { })
```

**§ 4(4).** An explicit payload-discarding or empty field form suppresses **this particular omitted-payload information diagnostic**:

```sec
Err(.NotFound { }) => GetDefaultContent()
Err(.InvalidEncoding(_)) => GetDefaultContent()
```

The suppression is not a waiver of ownership, destructor or non-discardability obligations. A pattern or containing operation that cannot legally abandon, retain, borrow, destroy or transfer its payload is still rejected by the ordinary ownership rules; no implicit partial move, silent clone, or hidden ownership escape is authorized.

**§ 4(5).** This payload-awareness rule applies uniformly to ordinary union variants and concrete error variants in `try` and `match`. It does not change which variants the compiler regards as reachable.

---

## § 5. MD-029: the contextual `.Variant` error-pattern shorthand

**§ 5(1).** When the protected `try` failure channel or `Result[T, E]` match subject supplies **one unambiguous closed concrete error type `E`**, the programmer may omit its name and begin an error-variant selector with `.`:

```sec
Err(.NotFound)
Err(.NotFound(info))
Err(.NotFound { Path: path })
Err(.NotFound { })
```

These resolve exactly as the corresponding `Err(FileError.NotFound...)` forms when `E` is `FileError`.

**§ 5(2).** The unqualified form never searches for a conveniently named variant among imported or visible error types. The expected concrete error type must already be determined **without looking up the candidate variant name**. Presence of a uniquely spelled variant in the program does not establish a type context.

**§ 5(3).** For `Result[T, error]`, the open root `error` does not supply one concrete error type. The abbreviation is unavailable, even if only one currently known concrete type declares the variant or a particular flow seems to narrow the possibilities. State the concrete type explicitly:

```sec
Err(FileError.NotFound { Path: path }) => Log(path)
```

**§ 5(4).** A `try` protecting heterogeneous failure sources may use the abbreviation **only** if the existing resolved error context establishes the same one concrete error type for the pattern. The compiler must not infer a new common error type from one variant name or implicitly invent an error union. Where the context is heterogeneous or open, use a fully qualified pattern.

**§ 5(5).** The abbreviation is a pattern syntax, not a general expression constructor. It does not make `.NotFound` a globally valid value expression or change standalone `match` variant spelling outside this correction. A fully qualified pattern remains valid and is never deprecated.

**§ 5(6).** The shorthand is an additive feature targeted for **Sec 0.1.2 or 0.1.3**. Its implementation and activation version must be recorded explicitly in the relevant grammar, compiler capability and release tracking. The design decision does not assert present Sec 0.1 compiler support.

---

## § 6. MD-029: open error narrowing and `try`/`match` coverage

**§ 6(1).** When `Result[T, error]` holds a concrete `FileError`, a fully qualified error-variant pattern first checks the **concrete error identity**, then the **selected variant**, and only then exposes the variant payload fields/binding. It may not read the payload of a different error type or variant.

```sec
fn ReadFile() Result[string, error] {
    // Implementation omitted in this normative illustration.
}

// Illustrative handler over an open error channel.
let content := try ReadFile() {
    Err(FileError.NotFound { Path: path }) => {
        Log(path)
        GetDefaultContent()
    }
}
```

The body above is an illustration, not a compilable definition of `ReadFile` until a real body is supplied.

**§ 6(2).** The same narrowing applies to an exhaustive `match` with a mandatory open-error fallback unless a separately established valid flow proof narrows the error domain:

```sec
match result {
    Ok(content) => Use(content)
    Err(FileError.NotFound { Path: path }) => HandleMissing(path)
    Err(errorValue) => HandleOther(errorValue)
}
```

**§ 6(3).** For `Result[T, FileError]`, unguarded arms for every declared reachable `FileError` variant, together with the necessary `Ok` coverage, can exhaust the match. A bare payload-carrying variant counts as full variant coverage as specified in § 4(2).

**§ 6(4).** An unguarded `Err(errorValue)` or `Err(_)` covers **all remaining error cases**, so a subsequent specific error-variant handler is genuinely covered/unreachable and remains a compile-time error. A variant handler following an unguarded complete set of variant handlers is likewise covered. Guarded handlers are handled using the existing guard-false continuation rules.

**§ 6(5).** Pattern-bound payloads are visible only in their selected arm's guard and body. Ownership transfer for a move-only payload is committed only after a guard succeeds and its arm is selected. A guard-false path and an unmatched `try` propagation path must retain the original error value and obligations. A borrowed error cannot be silently consumed or yield a mutable borrow without authority.

**§ 6(6).** Error-pattern selection must preserve left-to-right evaluation and evaluate the original protected expression/subject exactly once. A synthesized check or pattern destructuring must not introduce extra getter calls, fresh allocation, dynamic reflection or a mandatory runtime service.

---

## § 7. MD-029: grammar amendment

**§ 7(1).** `rules/foundations/grammar.md` currently references `ErrPattern` without defining the nonterminal. The owning grammar must define it normatively. The following productions specify the *semantic grammar*, using existing `QualifiedIdentifier`, binding and shallow field-pattern productions, and may be translated into the grammar file's established notation without changing their meaning:

```text
ErrPattern
    ::= "Err" "(" ErrorSelection ")"

ErrorSelection
    ::= "_"
      | Identifier
      | QualifiedErrorVariant [ ErrorVariantPayloadPattern ]
      | ContextualErrorVariant [ ErrorVariantPayloadPattern ]

QualifiedErrorVariant
    ::= QualifiedIdentifier "." Identifier

ContextualErrorVariant
    ::= "." Identifier

ErrorVariantPayloadPattern
    ::= "(" PatternArgument ")"
      | "{" [ FieldPattern { "," FieldPattern } [ "," ] ] "}"
```

**§ 7(2).** `Identifier` in the unadorned error-binding position means a whole-error binding, not an implicitly inferred enum/union variant. A variant-only shorthand **always** has a leading dot. A qualified variant has a known type/variant boundary established by name resolution; no imported type-name guessing is introduced.

**§ 7(3).** Payload-pattern admissibility is determined by the resolved error variant: `(...)` for an appropriate single-payload binding, `{...}` for struct-like field destructuring, and no suffix for a variant-only match. Existing pattern-binding modes (`name`, `_`, `ref name`, `ref mut name`) are reused only where already legal for that payload kind.

**§ 7(4).** The grammar distinguishes the braces **inside** `Err(.NotFound { Path: path })` from the braces introducing a `try` handler list, a `match` body or an arm body. Formatter/CST recovery must not reinterpret either brace as a different construct.

**§ 7(5).** All the productions above remain confined to the error-specific refinement extension of the existing restricted match grammar; they do not add recursive `MatchPattern` as a general-purpose expression grammar.

---

## § 8. MD-042: reachable-state-based exhaustiveness

**§ 8(1).** An exhaustive `match` must cover every **semantically reachable state of the subject at the match point**. When authoritative analysis proves a smaller reachable variant set, only that set is needed for exhaustiveness.

```sec
type Figure union {
    Dot
    Line(int)
    Circle(int)
}

let figure := Figure.Dot

// Exhaustive because the subject is proven to be Figure.Dot here.
match figure {
    Dot => DrawDot()
}
```

**§ 8(2).** Validly typed arms for variants proven impossible **solely from the subject's current known state** remain **legal**. They may be intentionally retained for clarity, reviewability or a future broader value source. The compiler should produce a default **Information** diagnostic explaining the proof, not a mandatory error.

```sec
match figure {
    Dot => DrawDot()
    Line(length) => DrawLine(length)      // Legal; provably inactive here.
    Circle(radius) => DrawCircle(radius) // Legal; provably inactive here.
}
```

**§ 8(3).** An inactive arm must still be syntactically valid, refer to a declared variant, use a valid binding shape, type-check its guard/body and respect ownership/lifetime obligations. Being inactive is not permission to contain an invalid program fragment.

**§ 8(4).** Proven inactivity is **different** from genuine source-order redundancy. A duplicate or later arm fully covered by an earlier unguarded arm remains a **compile-time error** under `flowcontrol_match.md` § 25 and existing try-handler coverage rules:

```sec
match option {
    Some(value) => Handle(value)
    Some(value) => HandleAgain(value) // Error: the earlier arm covers it.
    None => HandleNone()
}
```

**§ 8(5).** Consequently the compiler distinguishes: (a) a structurally invalid/duplicated/covered pattern (Error), (b) a valid arm excluded only by a proven subject state (Information), and (c) an arm not proven excluded (no inactivity diagnostic). This classification does not depend on whether lowering later eliminates the inactive arm.

**§ 8(6).** A proof of the active variant must be sound for the **actual match subject**: immutable known value facts, trusted path refinements and any applicable compiler-known provenance. Mutation, reassignments, aliases, foreign calls, dynamic calls, property reads with unproved stability, invalidated flow assumptions and unknown results must conservatively widen or invalidate the proof according to the owning analysis. A compiler must not use the declaration's initial value after it may have changed.

**§ 8(7).** If no sound restricted-state proof exists, exhaustiveness uses the **whole domain** of the resolved subject type, including all legal union/enum/Result/Option cases, reachable `empty` state and unknown encodings of open bit-backed enums as defined by their owner books. Absence of proof is not a license to omit arms.

**§ 8(8).** Guard semantics remain unchanged. A guarded arm does not count as complete coverage merely because its variant is known; a false guard may leave a reachable value unmatched. Established narrow finite-domain guard proofs remain valid and are not generalized by this correction.

---

## § 9. MD-042: arithmetic errors and other failure provenance

**§ 9(1).** The **declared error type** and the **possible errors of a specific expression** are distinct semantic facts. Widening a core error enum does not retroactively make every operation capable of producing all newly declared variants.

**§ 9(2).** After the separately approved MD-067 correction, `ArithmeticError` includes specific decimal-related variants such as `PrecisionExhausted` and `ScaleOverflow`. An ordinary `int` division does **not** acquire decimal precision failure paths merely because it shares the same public error enum.

**§ 9(3).** A `try` whose failure facts are proven to arise **only** from an integer division may therefore include a legally typed `PrecisionExhausted` handler, which is **informationally inactive**, not invalid. A source expression with additional calls or failure-producing operands must incorporate every reachable failure source; it may not apply the integer division's restricted set to the entire protected subtree without proof.

```sec
// For integer a and b, PrecisionExhausted is not a failure
// of the division operation itself.
let quotient := try a / b {
    Err(ArithmeticError.DivisionByZero) => RecoverZero()
    Err(ArithmeticError.PrecisionExhausted) => RecoverPrecision()
    Err(ArithmeticError.Overflow) => RecoverOverflow()
}
```

**§ 9(4).** Conversely, a declaration such as `fn Calculate() Result[int, ArithmeticError]` does **not** prove that `PrecisionExhausted` is impossible. Its implementation may perform decimal arithmetic before producing an `int`. A caller may narrow its failures only using reliable exposed semantic facts; it must not inspect only the success type, function name or an imagined implementation.

**§ 9(5).** Failure-provenance facts may support a restricted active-error set in `try` or `match` **only when the complete relevant provenance is known and sound**. This is a semantic proof, not an ABI property, optimization heuristic or untracked inference from a coincidental variant name.

**§ 9(6).** This correction does not change ordinary `ArithmeticError` runtime behavior, error variants or decimal algorithms. Those are owned by the separate MD-066/MD-067/MD-068 decimal correction. This section specifies only their interaction with MD-042 reachability.

---

## § 10. MD-042: diagnostics, stable identity and developer assistance

**§ 10(1).** The source pattern location, resolved subject type, excluded variant and proof reason should be included in an inactive-arm **Information** occurrence. Where available, a related location may identify the assignment or prior condition establishing the active variant.

```text
Information: This match arm cannot be selected for the current subject state.

The value is proven to contain Figure.Dot at this match point.
Figure.Line remains a valid variant and this arm is legal.
```

**§ 10(2).** Existing `S3001` identifies an unreachable **statement**, while `S3003` is already reserved for an unreachable **arm**. Do **not** reuse a mandatory-error definition with its severity silently changed to Information. Allocate/verify an appropriate **new stable advisory diagnostic identity** through the existing diagnostic registry, or use an already compatible advisory identity if one demonstrably exists. Preserve the current identities and classifications of genuinely erroneous reachability diagnostics.

**§ 10(3).** The same principle governs § 4's omitted-payload Information diagnostic. Register a stable identity with canonical English message, structured arguments, default severity, optional help and appropriate LSP code. Both information occurrences are configurable as Sec's normal advisory system permits. Avoid diagnostic spam by grouping or deduplicating equivalent proof occurrences where the diagnostics framework prescribes it.

**§ 10(4).** Tooling may show an inactive-variant hint or an explicitly ignored payload without rewriting source. Automated quick fixes, if offered, must be safe, optional and driven by semantic facts. The formatter must never remove inactive arms or rewrite `Err(.Variant)` to a different variant because of this analysis.

---

## § 11. MD-102: contextual `Some(value)` in arguments

**§ 11(1).** `Some(value)` is the canonical **one-payload Option constructor**, when it resolves to the compiler-known `Option[T].Some` rather than a visible, ordinary callable named `Some`. It is valid as a call argument or other expression wherever the ordinary type-resolution rules can establish its `Option[T]` type.

**§ 11(2).** An expected `Option[T]` parameter may contextually type the payload. A bare `Some(value)` is not a function call to a separately declared global `Some` by default and must not produce `S1157` when valid Option context exists.

```sec
fn AcceptOption(value: Option[int]) void {
}

AcceptOption(Some(42))
```

**§ 11(3).** The payload contributes type information to **overload candidate elimination**, exactly like ordinary argument type information. A candidate expecting `Option[U]` is viable only if the payload can legally initialize `U` under Sec's existing contextual-literal, type-compatibility and explicit-conversion rules. The compiler must not choose a candidate merely because its parameter is an Option.

```sec
fn Store(value: Option[int]) void {
}

fn Store(value: Option[string]) void {
}

Store(Some(42)) // Selects the Option[int] overload.
Store(None)     // Ambiguous unless other arguments/context resolve it.
```

**§ 11(4).** Ordinary numeric-literal shaping and normal overload ranking remain authoritative. For multiple candidates such as `Option[int32]` and `Option[int64]`, infer the winner only if existing ranking rules determine it. Otherwise report ambiguity with both candidate types and an actionable suggestion for explicit typing. Do not invent a `Some`-specific candidate precedence.

**§ 11(5).** Type inference/overload resolution examines payload **types and static constraints**, not runtime payload values or side effects. The actual payload is evaluated exactly once, in the ordinary call evaluation order. A failed overload choice must not perform runtime evaluation.

---

## § 12. MD-102: generics and type inference

**§ 12(1).** A payload in `Some(value)` may establish the generic type parameter `T` in an expected `Option[T]`, provided the payload has enough type information under ordinary inference rules:

```sec
fn Process[T](value: Option[T]) void {
}

Process(Some(42)) // T = int.
```

**§ 12(2).** Generic inference considers **all supplied arguments and constraints**. A payload-derived inference must agree with every other inference source, explicit generic parameter, required interface or contract, and selected overload; otherwise the invocation is rejected with a focused incompatibility diagnostic.

```sec
fn Unwrap[T](value: Option[T], fallback: T) T {
    match value {
        Some(item) => return item
        None => return fallback
    }
}

let result := Unwrap(Some(42), 0) // T = int.
```

**§ 12(3).** `None` remains different: it carries no payload from which to infer `T`. `Process(None)` cannot infer `T` without other usable context. `Unwrap(None, 0)` may infer `T` from the fallback argument, subject to the existing compatibility and contextual-inference rules.

**§ 12(4).** A type-inferred `Some(value)` is **not** permission to violate ownership: no implicit move from a reusable non-copyable Place, no invented copy, no concealed reborrow, no lifetime extension, no mutable borrow without authority, and no escape of a short-lived reference. A successful generic substitution is only a typing result; construction and call transfer must pass the usual separate ownership/borrow/lifetime checks.

**§ 12(5).** Generic inference must not turn an independently unknown payload (for example bare `None`, empty `[]` or `null` without an appropriate expected type) into an arbitrary chosen `T`. It may infer when some other valid parameter/context provides the missing expected type; it must otherwise explain the missing evidence.

---

## § 13. MD-102: standalone `Some` type inference

**§ 13(1).** Where the payload expression has an **independently inferable type `T`**, an uncontextualized `Some(value)` has the nominal type **`Option[T]`**. This is an approved positive inference rule, not an implicit optional conversion of unrelated expressions.

```sec
let number := Some(42)     // Option[int].
let name := Some("Alice")  // Option[string].
```

**§ 13(2).** An explicit expected `Option[T]` type wins as the context in which to validate and shape the payload:

```sec
let number: Option[int64] := Some(42)
```

The inferred form must not bypass existing exact literal representability, nominal identity, contracts or special type restrictions.

**§ 13(3).** Standalone `None` still lacks a unique type and requires an expected `Option[T]` context:

```sec
let invalid := None // Error: Option type cannot be inferred.
```

**§ 13(4).** An uncontextualized nested `Some(None)` also lacks the inner `Option[T]` type and must be diagnosed. It becomes valid when a sufficient expected type is supplied for every otherwise unknown nested constructor:

```sec
let nested: Option[Option[int]] := Some(None)
```

**§ 13(5).** A nested `Some(Some(value))` may be typed by repeated application of the same rule if its innermost payload is independently inferable; this does **not** introduce general nested match-pattern syntax.

**§ 13(6).** Existing name-resolution and shadowing rules apply. If a lexically visible user callable named `Some` is selected by ordinary lookup, the expression is analyzed as that call, not silently replaced with an Option constructor. Compiler-specific contextual constructor recognition must not hijack a valid user binding or create shadow symbols. When required, explicitly typed `Option[T].Some(...)` construction remains a way to identify the intended variant under its owning construction rule.

---

## § 14. MD-102: invalid `None()` and diagnostic accuracy

**§ 14(1).** The Option absence variant is spelled **`None`**, not `None()`. Calling the compiler-known absence variant with empty parentheses is invalid:

```sec
let value: Option[int] := None    // Valid.
let invalid: Option[int] := None() // Invalid Option construction.
```

**§ 14(2).** The existing `S1157` pathway should retain its established stable identity and clearly explain that the compiler-known `None` variant takes no arguments and is not a function. Verify the exact registered diagnostic definition and only amend its wording/help under the established catalog contract; do not invent a second ID for the same failure. Do not falsely label a legitimate user-defined function as the Option absence variant.

**§ 14(3).** A contextual `None`, `Some(...)`, `[]` or `null` passed to a parameter outside its accepted type family remains a type error, not silent acceptance. Preserve the existing fixes and tests for this issue.

---

## § 15. Semantic analysis and source-fact ownership

**§ 15(1).** Sema must resolve a precise immutable error-pattern plan containing the protected carrier/failure context, concrete error identity where applicable, variant identity, payload shape, selected field bindings and borrowing/transfer modes, guard presence and arm coverage. The backend must not rediscover variant names by string comparison or re-infer the source error type from an ABI layout.

**§ 15(2).** For MD-042, Sema must retain the distinction between **declared possible variant domain**, **soundly proven reachable subset**, and **prior-arm coverage**. Proven inactivity does not mark an arm invalid, while duplicate/covered-arm diagnostics retain their usual error semantics. When a proof depends on aliasing, control-flow state or a callable failure provenance, invalidate/recompute it at the relevant mutation or unknown-effect boundary.

**§ 15(3).** For MD-102, overload and generic inference must treat a `Some` payload as a type-bearing expression while preserving its candidate-specific expected type, literal shaping, provenance and ownership obligations. Selection must be deterministic and independent of declaration order. The chosen concrete `Option[T]` and payload construction must be recorded before IR building.

**§ 15(4).** The parsed contextual form, selected qualified variant and payload-binding sites must retain source locations for hover, diagnostics, references and go-to-definition. The LSP must offer completions from the known concrete error type for `Err(.`, without falling back to a workspace-wide name search. For open `error` channels, request or suggest a qualified concrete type.

**§ 15(5).** Both semantic IR and target lowering must preserve first-match selection, guard-false continuation, exactly-once subject/payload evaluation, correct copy/move/borrow actions, destruction obligations and residual `try` propagation. Unavailable backend functionality must be reported as an explicit capability limitation rather than implemented through wrong semantics or undefined payload reads.

---

## § 16. Required canonical rulebook integration

**§ 16(1).** Amend the following **existing** normative owners without renumbering cited paragraphs:

| Owner | Required amendment |
|---|---|
| `rules/foundations/grammar.md` | Define `ErrPattern` completely; accept payload-bearing error refinement, shallow fields and `Err(.Variant...)` only in the approved context; preserve brace/parenthesis roles. |
| `rules/errors/errorhandling.md` | Extend §§ 15–20 and 27 for payload-bearing variant patterns, source-order selection, guards, ownership, open-error narrowing, partial propagation and shorthand legality. |
| `rules/control-flow/flowcontrol_match.md` | Extend §§ 9–15 and 28 for error payload patterns and the bare-variant rule; amend §§ 5 and 25 for valid proven-inactive arms versus genuinely covered/duplicate arms. |
| `rules/declarations/unions.md` | Permit bare variant matching for payload-bearing ordinary unions and define explicit empty field/no-binding behavior without hidden partial moves. |
| `rules/types/types.md` | Expand contextual `None` and empty-literal rules to `Some`; define self-inferred `Option[T]` and the continued lack of standalone `None` type. |
| `rules/declarations/functions.md` | Establish overload candidate filtering from `Some` payloads, deterministic resolution and ordinary call ownership. |
| `rules/declarations/generics.md` | Establish the `Option[T]` payload-to-`T` inference rule, interplay with other arguments and constraint checks. |
| `rules/tooling/diagnostics.md` | Allocate/verify stable Information diagnostics; preserve existing mandatory unreachable and syntax IDs; include mentor explanations and LSP parity. |
| `rules/compiler/semantic_ir.md` | Preserve resolved pattern and coverage facts, type/variant identities, try residuals, ownership and proof provenance. |
| `rules/tooling/lsp.md`, `rules/tooling/formatter.md` | Contextual variant completion, hover, canonical source preservation, code-action safety, deterministic idempotent formatting. |

**§ 16(2).** Related existing compiler/regression boundaries must be checked, including `internal/parser/expression.go`, `internal/sema/analyzer_try_handlers.go`, `internal/sema/semantic_facts.go`, `internal/sema/match_field_patterns.go`, `internal/sema/null_sentinel.go`, `internal/sema/analyzer_inference.go`, `internal/ir/semantic/`, `internal/lowering/`, and `internal/diagnostics/`. These are investigation anchors, not a claim that one particular file must own each implementation detail.

**§ 16(3).** The MD-029 implementation must not erase or weaken the earlier MD-008 exception for `Err(ConcreteError.Variant)` or the closed versus open error-channel distinction. The MD-042 implementation must not reclassify ordinary `S3001`/`S3003` errors as information. The MD-102 implementation must not regress the previously implemented MD-027 argument-context deferral and same-family validation.

---

## § 17. Comprehensive conformance and regression matrix

**§ 17(1).** Add focused parser, Sema, Semantic IR and end-to-end tests for each positive and negative boundary. At minimum, tests must cover the following individually identifiable cases. Test labels below are **test requirements**, not new governance IDs.

| Test | Requirement |
|---|---|
| `M029-01` | Closed concrete error enum: qualified specific variant, whole-error binding and `Err(_)` retain existing meaning. |
| `M029-02` | Closed union error: `Err(Error.Variant(info))` binds one payload and exposes correct type. |
| `M029-03` | Struct-like union error: `Err(Error.Variant { Field: name })` and partial fields bind exactly chosen fields. |
| `M029-04` | Variant-only `Err(Error.Variant)` on a payload-bearing variant is valid; Information cites its payload. |
| `M029-05` | Explicit `Err(Error.Variant { })` / appropriate `Variant(_)` suppresses the omitted-payload information, subject to discardability. |
| `M029-06` | Bare payload-bearing ordinary union variant pattern behaves the same; complete coverage and information verified. |
| `M029-07` | `Err(.Variant)`, `Err(.Variant(info))`, `Err(.Variant { Field: field })` resolve from an unambiguous closed error type. |
| `M029-08` | `Err(.Variant)` is rejected for `Result[T, error]` and heterogeneous/ambiguous error contexts, even if only one type declares that spelling. |
| `M029-09` | Fully qualified payload-bearing pattern narrows open `error` by concrete identity and variant before field access. |
| `M029-10` | Open `Result[T, error]` exhaustive match requires explicit remaining-error coverage; partial `try` permits residual propagation. |
| `M029-11` | Invalid variant, missing field, duplicate field, wrong binding mode and illegal recursive pattern produce focused diagnostics. |
| `M029-12` | Guard false preserves error/payload and residual ownership; selected arm commits legal transfer once; failed branch does not consume. |
| `M029-13` | Shared and mutable reference patterns obey borrow/alias rules; move-only fields are never silently partially moved. |
| `M029-14` | No extra subject evaluation or double-destruction; handling of non-discardable payloads is not silently weakened. |
| `M029-15` | Try-handler braces and field-pattern braces parse/recover distinctly; formatter is idempotent. |
| `M042-01` | Proven immutable `Figure.Dot` permits a match containing only `Dot` as exhaustive. |
| `M042-02` | Additional valid `Line`/`Circle` arms are legal and produce Information, not Error. |
| `M042-03` | An unguarded duplicate, a previously covered arm, or an arm after catch-all remains an error with related location. |
| `M042-04` | An inactive arm with an invalid field, type, guard or ownership action still fails semantic checking. |
| `M042-05` | Mutation/alias/unknown return invalidates variant proof; missing coverage is then a compile-time error. |
| `M042-06` | Guard-false behavior, reachable `empty`, bit-backed open enum values and enum aliases preserve their existing coverage laws. |
| `M042-07` | Integer-only arithmetic provenance excludes decimal precision errors; no false exclusion for `Result[int, ArithmeticError]` from unknown function implementation. |
| `M042-08` | CLI, JSON and LSP report the same advisory diagnostic identity and proof information. |
| `M102-01` | `Store(Some(42))` selects `Option[int]` over `Option[string]`; `Store(None)` remains ambiguous. |
| `M102-02` | Fixed-width numeric alternatives use ordinary literal ranking and report ambiguity when no unique winner exists. |
| `M102-03` | `Process[T](Option[T])` accepts `Process(Some(42))` with inferred `T = int`. |
| `M102-04` | `Unwrap(Some(42), 0)` infers compatible `T`; incompatible argument constraints are rejected. |
| `M102-05` | `Unwrap(None, 0)` may infer from fallback; `Process(None)` cannot invent `T`. |
| `M102-06` | `let v := Some(3)` and `let s := Some("text")` infer `Option[int]` and `Option[string]`. |
| `M102-07` | Explicit `Option[int64]` expected type shapes `Some(42)` correctly and enforces representability. |
| `M102-08` | `let x := None` and uncontextualized `let x := Some(None)` fail; explicitly typed nested Option succeeds. |
| `M102-09` | `Some(Some(1))` infers nested Option without introducing a nested pattern feature. |
| `M102-10` | Move-only source requires legal transfer; references require valid lifetimes, mutation authority and borrow independence. |
| `M102-11` | A visible callable named `Some` is not silently rewritten as Option construction; ordinary name resolution is honored. |
| `M102-12` | Built-in `None()` receives the existing `S1157` guidance and `None` without parentheses remains legal in context. |
| `M102-13` | Invalid None/Some/[]/null argument family is rejected, never silently accepted. |
| `M102-14` | Contextual expressions are evaluated once; compiler facts, formatter and LSP reflect the selected type and constructor. |

**§ 17(2).** For each test that requires execution or native lowering, run every currently supported target/CompilationPlan relevant to the semantic feature, and report explicit planned/unsupported lowerings separately. Never mark a source-valid feature semantically invalid just because one target backend is not yet capable of emitting it.

**§ 17(3).** Fuzz/property-based and differential tests are appropriate for error-pattern variant selection and coverage. For finite concrete error domains, compare the Sema coverage result against an exhaustive independent variant enumeration. For open error channels, include unrelated concrete error types sharing the same variant spelling.

**§ 17(4).** Add one cross-feature regression where a `Some` value is used inside a `Result`/error recovery flow, ensuring that nested expressions do not accidentally enable nested match patterns or bypass try propagation or ownership checks.

---

## § 18. Sequential implementation and verification plan

**§ 18(1) — Phase 1: baseline and change reconciliation.** Record actual checked-out HEAD, worktree state and currently applicable canonical rules. Inventory existing parser/Sema/IR tests and governance integration IDs. Confirm MD-029/042/102 are not already integrated by a newer patch. Determine which code paths need change without rewriting known-good behavior. Run existing focused tests as baseline. **Stop** on unrelated pre-existing regressions and report them separately.

**§ 18(2) — Phase 2: grammar and source AST.** Implement MD-029 payload/error shorthand grammar and variant-only pattern parsing, preserving existing `match` and `try` syntax/recovery. Implement only this stage, add `M029` syntax tests, run parser and formatter-focused tests, and fix failures before proceeding.

**§ 18(3) — Phase 3: MD-029 frontend semantics and ownership.** Resolve concrete/open error identities, shallow field binding, variant-only coverage, payload-awareness Information, guard-false preservation and ownership. Add Sema tests for closed/open channels, invalid nested forms, non-discardability and source-order shadowing. Do not proceed until these pass.

**§ 18(4) — Phase 4: MD-042 state-proof model.** Distinguish declared variant domain, reachable subset and source-order pattern coverage in authoritative Sema facts. Implement advisory inactive arms while retaining mandatory duplicate/covered-arm errors. Add failure-provenance and mutation-invalidating tests. Verify diagnostics and ownership checks on inactive arms before proceeding.

**§ 18(5) — Phase 5: MD-102 contextual resolution.** Implement payload-aware overload filtering, generic inference, standalone `Option[T]` inference, selected constructor facts and existing symbol precedence. Validate `None()` help, literal shaping, missing type evidence, call order and copy/move/borrow/lifetime checks. Run focused types, functions and generic Sema suites.

**§ 18(6) — Phase 6: Semantic IR, MLIR and lowering.** Preserve new resolved facts through canonical Semantic IR and the maintained Sec MLIR track, with exact-once evaluation and ownership. Verify IR and dialect invariants, target capability boundaries and end-to-end fixtures. Do not claim successful lowering where only Sema/parser tests pass.

**§ 18(7) — Phase 7: diagnostics, LSP and formatter.** Integrate stable advisory IDs and mentor explanations, precise source ranges, related locations, contextual completions, hover facts, fixes if safe, and idempotent formatting. Run CLI/JSON/LSP parity checks and existing diagnostic catalog validation.

**§ 18(8) — Phase 8: canonical documents and governance synchronization.** Amend all normative owners in § 16 in place, preserving cited numbers, source comments, examples and historical decision references. Reconcile existing YAML IDs; update `missing-decisions.yaml`, `language-rulebook-status.md`, existing TODO/package dependency references and test evidence. Move/categorize this correction under the repository's actual correction workflow only after integration is verified. Mark decisions *decided* and implementation *implemented* **independently**, never as one implied status.

**§ 18(9).** At every phase: run the narrowest relevant test suite; inspect, fix and rerun failures; run agreed broader suites at integration boundaries; record actual commands and results. A failed test is not satisfied by omitting it, renaming it or claiming an unsupported target succeeds.

---

## § 19. Governance, publication and closure criteria

**§ 19(1).** The original `missing-decisions.yaml` entries `MD-029`, `MD-042` and `MD-102` are **owner-decided as of 2026-10-28**. Their current `status: open` in the checked-out baseline is stale design tracking, not evidence of an unresolved question. Reconcile in place to the repository's established decided-but-implementation-pending status convention; preserve the three IDs and relevant historical descriptions/links.

**§ 19(2).** Reuse, extend or add requirements beneath existing governance integration IDs, notably `frontend.errorhandling-v2`, `frontend.match-v2`, and `frontend.types-core`; relevant diagnostics, generics, parser, Semantic IR, MLIR and tooling entries must be reconciled with their actual owners. New stable **subrequirement IDs**, if used, must follow the local machine-readable governance schema, with explicit implemented/remaining/identified-bugs states where that schema requires them. Do not generate parallel duplicate top-level integrations solely because this correction spans three decisions.

**§ 19(3).** Already-implemented behavior—including MD-008 variant matching, MD-027 contextual `None`/`[]`, MD-028 failure-set provenance, existing enum/union ownership and protected `try` expression evaluation—must be retained and regression-tested. Existing implementation claims must not be restated as new work; genuinely incomplete support must not be mislabeled implemented.

**§ 19(4).** The correction is ready for normative integration when all ambiguous choices in this document are resolved by its rules and existing owners, the canonical books contain the approved text without paragraph renumbering, and governance acknowledges the owner decision. **Implementation closure** additionally requires passing test evidence at each relevant source, Sema, IR, lowering and tooling boundary; release activation of `Err(.Variant...)` must record whether it ships in Sec **0.1.2 or 0.1.3**, not silently claim both.

**§ 19(5).** No additional owner decision is required to implement consequences already fixed by this document or by existing Sec rules. Raise a *new* unresolved decision only if implementation exposes a real semantic contradiction, incompatible prior approval, or previously unspecified fork that changes language behavior. Do not stop normal implementation merely to ask approval for routine consequences.

---

## § 20. Final owner-approved decision summary

**§ 20(1) — MD-029 (decided).** `Err` patterns may bind one payload or shallow fields of a concrete union-error variant in both `try` and `match`, including fully qualified narrowing from an open `error` channel. A bare variant pattern matches a payload-bearing variant without extraction and should produce Information about omitted data; explicit ignore suppresses only that information. The `Err(.Variant...)` form is valid only with an already unambiguous closed concrete error type and targets Sec 0.1.2/0.1.3. General recursive patterns are not introduced.

**§ 20(2) — MD-042 (decided).** A `match` is exhaustive over states soundly proven reachable. Extra correctly typed arms excluded only by such proof are permitted and may receive Information. Truly duplicate, source-order-covered or invalid arms remain errors. A larger declared error enum does not mean every operation can produce every variant; the compiler must distinguish type domain from proven operation-specific failure provenance.

**§ 20(3) — MD-102 (decided).** `Some(value)` contributes payload type information to ordinary overload selection and generic inference, and `Some(value)` independently has type `Option[T]` when `T` can be inferred from its payload. `None` requires an expected type. All ownership, borrowing and lifetime rules remain binding. `None()` remains invalid for compiler-known Option absence with the existing `S1157` mentor guidance.

**§ 20(4).** These are complete normative design decisions, **not a claim of implemented support** in the verified repository baseline. Publication as an applied fix requires the § 18–19 reconciliation and evidence.
