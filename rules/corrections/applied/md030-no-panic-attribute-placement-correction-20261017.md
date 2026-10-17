# MD-030 — `@noPanic` Attachment to Callable Bodies

**Status:** Applied 2026-10-17 — integrated into `rules/foundations/attributes.md` ("Callable-body attachment (MD-030)", "Allowed targets for verified effects", "Allowed attachment level"), `rules/errors/panic.md` § 21(8), `rules/foundations/grammar.md` (`CallableBodyAttributes`, `LambdaLetDeclaration`), `rules/declarations/properties.md` § 14, `rules/declarations/impl.md` § 19 and `rules/declarations/lambda-functions.md` § 32. Implemented: parser attachment and rejections (`internal/parser/attributes.go`, `parser.go`, `expression.go`), AST fields, Sema verification of init/free/accessor/lambda bodies (`internal/sema/analyzer_no_panic.go`). Tests `TestNoPanicAttachesToCallableBodies`, `TestNoPanicRejectsShortcutsAndIndirectForms`, `TestNoPanicCallableBodies`. Governance `frontend.panic`. MD-030 removed from `missing-decisions.yaml`. Known holes (verified 2026-10-17): LSP completion/hover targets, Semantic IR contract on accessor/lambda bodies, and the § 9(6) early-return/cleanup tests.  
**Created:** 2026-10-17  
**Updated:** 2026-10-17  
**Revision:** 1.0  
**Sec version:** 0.1  
**Canonical path:** `rules/corrections/applied/md030-no-panic-attribute-placement-correction-20261017.md`  
**Replaces:** No previously numbered paragraph. Amend the existing rulebooks in place without renumbering their paragraphs.  
**Decision ID:** MD-030  
**Primary rulebooks:** `rules/foundations/attributes.md`, `rules/errors/panic.md`, `rules/foundations/grammar.md`  
**Affected declarations:** `fn`, `unsafe fn`, `extern fn` where permitted by foreign-contract rules, `init`, `free`, property `get`, `set`, `try set`, and a directly lambda-initialized `let`  
**Related rulebooks:** `rules/declarations/impl.md`, `rules/declarations/properties.md`, `rules/declarations/lambda-functions.md`, `rules/memory/destruction.md`, `rules/tooling/formatter.md`, diagnostics, Semantic IR, testing, and governance.

---

## § 1. Scope and decision status

**§ 1(1).** This correction closes the *language-design placement question* recorded by MD-030. It specifies how the existing verified `@noPanic` attribute attaches to a callable body that is not an ordinary named function.

**§ 1(2).** The meaning of `@noPanic` remains the transitive, compiler-verified guarantee defined by `rules/errors/panic.md` § 21 and `rules/foundations/attributes.md`. This correction is **not** a new panic suppression, exception-catching, unchecked promise, new function-type syntax, or relaxation of mandatory safety verification.

**§ 1(3).** A locked design decision is not proof of implementation or validation. MD-030 must remain in the missing-decisions backlog until the owning canonical rulebooks have incorporated this correction and affected governance has been synchronized, according to the backlog removal rule. Compiler, formatter, LSP, and test conformance must be tracked separately.

## § 2. Single attachment principle

**§ 2(1).** A callable declaration that has an independently analyzable body may carry `@noPanic` on the line(s) **immediately preceding that declaration or callable member**, subject to the permitted targets in § 3.

**§ 2(2).** The attribute applies to the designated callable's invocation/body contract, **not** to the containing `impl`, the containing `property`, a preceding declaration, or sibling callable bodies.

**§ 2(3).** The attribute is written on a separate line, in the ordinary canonical attribute style. Whitespace and comments may intervene only under the same attachment rules as ordinary declaration attributes. The formatter must not silently relocate it to another semantic target.

**§ 2(4).** The same placement principle applies to already-supported named functions and methods; no new declaration syntax is required for those forms.

## § 3. Valid body-bearing targets

**§ 3(1).** In Sec 0.1, `@noPanic` is permitted immediately before a body-bearing `init` declaration inside a valid `impl`.

**§ 3(2).** It is permitted immediately before a body-bearing `free` lifecycle declaration inside a valid `impl`. It does not turn `free` into an ordinary function or allow it to return a value. Existing destructor and automatic field-cleanup obligations remain in force.

**§ 3(3).** It is permitted immediately before each individual property accessor: `get`, `set`, and `try set`. An annotated accessor is checked on its own body and reachable effects, including any required cleanup.

**§ 3(4).** The attribute is **not** placed on `property` as a shortcut for annotating every accessor; that attachment form is not introduced by this decision. Other accessors of the same property do not silently acquire `@noPanic`.

**§ 3(5).** Existing attachment to `fn`, `unsafe fn`, and other callable declarations remains subject to the attribute, unsafe, and FFI rules. A declaration's syntax being eligible for an attribute does not override ABI, foreign-effect, or proof obligations.

### Example: constructor and property accessors

```sec
// The type and members are illustrative; only existing Sec declaration forms are used.
type Counter struct {
    _value: int,
}

impl Counter {
    @noPanic
    init(value: int) {
        self._value = value
    }

    property Value: int {
        @noPanic
        get {
            return self._value
        }

        @noPanic
        set value {
            self._value = value
        }
    }
}
```

**§ 3(6).** A fallible setter retains its declared error channel under the canonical property rules. `@noPanic` constrains *panic*, not the normal return of a recoverable error.

The `try set` declaration keeps its existing explicit error-type syntax. A recoverable `Err(...)` result does not itself violate `@noPanic`; the body and any cleanup must still be proved panic-free. No new error type or setter signature is introduced here.

### Example: destructor

```sec
impl Resource {
    @noPanic
    free {
        // Only operations proven panic-free are permitted here.
        // Compiler-required field cleanup must also satisfy the guarantee.
    }
}
```

**§ 3(7).** In this example, `free` is legal only if the full destruction behavior, including any generated cleanup, meets the existing `noPanic` proof contract. An early bare `return` allowed by the separate MD-025 decision does not skip required cleanup.

## § 4. Lambda declarations: selected canonical spelling

**§ 4(1).** For a lambda assigned **directly** by a `let` binding, Sec 0.1 permits `@noPanic` immediately before the `let` declaration.

**§ 4(2).** The attribute's semantic target is the lambda's callable **body**, not the storage/binding and not an arbitrary initializer expression. This is a narrowly defined declaration-attachment exception to the existing ban on attributes on arbitrary local statements and expressions.

**§ 4(3).** The canonical spelling is:

```sec
let factor := 2

@noPanic
let multiply := capture(factor) fn(value: int) int {
    return value * factor
}
```

**§ 4(4).** § 4(3) demonstrates the *attachment syntax only*. The body is **not automatically proven `noPanic`**: checked integer multiplication may panic on overflow unless the required range/proof or an explicit fallible handling path rules it out. A compiler must diagnose a failed guarantee rather than silently accept the attribute.

**§ 4(5).** A straightforward proof-friendly example is:

```sec
@noPanic
let identity := fn(value: bool) bool {
    return value
}
```

**§ 4(6).** Both a capturing and a noncapturing direct lambda initializer are eligible. A type annotation on the `let` does not change its attribute target. The resulting lambda carries its proven invocation contract through ordinary identity, move, and use, without this correction inventing a new general effect-qualified `fn(...)` type grammar or weakening compatibility checks.

## § 5. Lambda eligibility and excluded syntax

**§ 5(1).** This decision does **not** authorize `@noPanic` on an arbitrary `let`, nor on an initializer whose lambda origin is hidden behind a function call, conversion, conditional, or other expression.

```sec
@noPanic
let operation := MakeOperation() // ERROR: no directly attached lambda body.
```

**§ 5(2).** The following expression-prefix form is **not** the canonical Sec 0.1 spelling introduced by this decision:

```sec
let operation := @noPanic fn(value: bool) bool {
    return value
}
```

**§ 5(3).** Anonymous lambdas passed directly to a call or returned without a binding may still be **inferred** panic-free if the semantic rules prove it; this correction does not provide explicit source attribute placement on such expressions. Their future explicit contract syntax is not settled here.

**§ 5(4).** No generic local-attribute feature, block attribute, attribute on every expression, `@noPanic impl`, or `@noPanic property` shortcut is added by MD-030. The independent question of effect-qualified interface requirements and generic constraint method declarations remains in MD-053.

## § 6. `noPanic` proof obligations

**§ 6(1).** All allowed placements invoke the **existing** transitive panic-effect checker. Reachable checked arithmetic, index bounds, foreign calls, compiler/runtime helpers, assertions, indirect calls, `defer`, destructors, and required cleanup must be accounted for under the owning panic and effect rules.

**§ 6(2).** A getter or setter with `@noPanic` does not make the other accessor panic-free. A callable whose proof fails remains a compile error on that declaration; the compiler must not treat the attribute as an advisory INFO/WARNING.

**§ 6(3).** Creation of a capturing lambda and its subsequent invocation are distinct operations. The annotation established in § 4 guarantees the lambda's **call**, not that every computation supplying the initializer, capture list, or containing declaration is panic-free. Captured-value ownership and cleanup obligations remain governed by existing closure and destruction rules.

**§ 6(4).** Unknown effect facts are not proof. Existing FFI and `unsafe` trust-boundary requirements remain authoritative. Nothing in MD-030 changes the conclusions of MD-059 or MD-060.

## § 7. Grammar and AST integration

**§ 7(1).** `grammar.md` must admit an optional applicable attribute sequence at the following *existing* declaration boundaries: `init`, `free`, `get`, `set`, and `try set`. The grammar must preserve that the attribute attaches to the respective callable member, not its enclosing property or implementation.

**§ 7(2).** The `let` grammar must permit the § 4 attribute sequence only where its initializer is syntactically a direct `LambdaExpression`, including an optional `CaptureClause`. No new lambda introducer is defined.

**§ 7(3).** AST and Sema must store the attribute and verified guarantee on the actual callable identity/body. The enclosing `let` is only a source attachment surface in the lambda case. Parsing or lowering must not turn this into a blanket attribute on a local variable or into an unverified effect assertion.

**§ 7(4).** Preserve distinct effect identities of property getter, infallible setter, fallible setter, constructor, destructor and lambda invocation; never merge them solely because they share a containing declaration.

## § 8. Diagnostics, formatting, and LSP

**§ 8(1).** Invalid attachment must identify the attribute, the syntactic target, and the closest supported placement; diagnostics must not pretend the only legal site is a top-level `fn`.

**§ 8(2).** A proof failure must identify the actual panic-capable operation or unresolved dependency, including transitive call or cleanup paths where feasible, and explain how to make the guarantee true (prove bounds, handle explicit fallibility, change the operation, or remove the guarantee).

**§ 8(3).** The formatter writes each declaration attribute on its own line before the declaration, retains attachment across whitespace/comments as already specified, and does not move `@noPanic` inside the lambda expression.

**§ 8(4).** LSP hover/diagnostics distinguish `@noPanic` on the callable body from attributes on a containing `let`, `property`, or `impl`, and show the real Sec callable declaration and effect evidence where available.

## § 9. Required conformance and regression tests

**§ 9(1).** Provide accepted parsing and semantic tests for `@noPanic` immediately before: ordinary named function, `init`, `free`, property `get`, property `set`, property `try set`, noncapturing lambda `let`, capturing lambda `let`, and lambda `let` with an explicit callable type annotation.

**§ 9(2).** Provide negative attachment tests for: arbitrary `@noPanic let` that does not directly initialize a lambda; `@noPanic property` as an all-accessor shortcut; `@noPanic impl` as an all-member shortcut; and the unsupported inline expression-prefix lambda form.

**§ 9(3).** Prove that annotating the getter does not annotate the setter; likewise for the reverse. A fallible setter returning a declared `Err` is allowed under `@noPanic` when its panic effects are otherwise proven absent.

**§ 9(4).** Verify failure on checked multiplication that might overflow, panic-capable indexing, unproven foreign effects, panic-capable callees, reachable explicit panic, and panic-capable cleanup. Do not weaken the diagnostic when the body is a lambda, constructor, destructor or accessor.

**§ 9(5).** Verify that formatter roundtrips preserve the attribute's target, and that LSP hover and Semantic IR retain the verified contract on the callable rather than merely the binding.

**§ 9(6).** Test a constructor and destructor containing early `return` paths under existing flow/destruction contracts, including generated field cleanup, without allowing an early return to evade a `noPanic` obligation.

## § 10. Normative rulebook integration and governance

**§ 10(1).** Update `rules/foundations/attributes.md` to document the allowed additional attachment levels and the narrowly scoped lambda-`let` exception; retain existing general constraints for unrelated attributes.

**§ 10(2).** Update `rules/errors/panic.md` § 21 to point to the placement rule and clarify that it applies to all eligible callable bodies without changing the proof contract.

**§ 10(3).** Update `rules/foundations/grammar.md`, `rules/declarations/properties.md`, `rules/declarations/impl.md`, and `rules/declarations/lambda-functions.md` only where needed for exact source productions, attachment targets and callable/lambda identity. Keep all previously published normative paragraph numbers stable.

**§ 10(4).** Reconcile affected governance fragments against existing IDs and actual compiler/test state. Do not reclassify a design decision as implemented/validated merely because this correction exists. Update the authoritative missing-decisions index only after normative integration and governance synchronization are complete.

**§ 10(5).** This correction intentionally does **not** decide general attributes on lambdas as expressions, effect-qualified function types, interface effect requirements (MD-053), or Sec 0.2 linear callable contracts.

---

## References (canonical source paths)

- `rules/foundations/attributes.md` — attribute attachment; `@noPanic` verified guarantee; formatter.
- `rules/errors/panic.md` — § 21 transitive `@noPanic` proof.
- `rules/foundations/grammar.md` — declaration, lambda expression and capture-clause forms.
- `rules/declarations/properties.md` — getter, setter and fallible setter forms.
- `rules/declarations/lambda-functions.md` — lambda declarations and capture semantics.
- `rules/declarations/impl.md` — `init` and `free` placement.
- MD-030 in `missing-decisions.yaml` (snapshot dated 2026-10-16).
