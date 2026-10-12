# MD-028, MD-032 and MD-055 — `try` Expression Coverage and Formatter Layout

- **Status:** Applied 2026-10-12 to `rules/errors/errorhandling.md` (§§ 11, 11.2, 13.1), `rules/errors/runtime_checks.md` ("`try` boundaries") and `rules/tooling/formatter.md` (§ 13(11)–(14), § 18(7), § 18(10a)); switch examples of `rules/control-flow/flowcontrol_switch.md`, `rules/control-flow/flowcontrol_while.md` and `rules/foundations/grammar.md` follow the decided label layout; lowering of intermediate chain receivers remains tracked in governance
- **Created:** 2026-10-12
- **Last updated:** 2026-10-12
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Proposed canonical path:** `rules/corrections/pending/md028-md032-md055-try-switch-import-comments-correction-20261012.md`
- **Decision IDs:** `MD-028`, `MD-032`, `MD-055`
- **Amends:** `rules/errors/errorhandling.md`, `rules/errors/runtime_checks.md`, `rules/tooling/formatter.md`
- **Implementation governance:** `governance/errors.yaml`, `governance/formatting.yaml`; synchronize any additional registry fragments that track the affected features
- **Reference:** Latest supplied `missing-decisions.yaml`, updated 2026-10-09
- **Replaces:** None. Preserve published paragraph identifiers when applying this correction.

## 1. Scope and authority

**1.1.** This correction records three decisions by the language owner. It clarifies existing `try` semantics and resolves two formatter-layout questions. It does not introduce a new error type, new error wrapper, new indentation width, or a new import-sorting algorithm.

**1.2.** Amend the owning rulebooks without renumbering previously published paragraphs. Add local numbered paragraphs or replace contradictory prose and examples in place. Preserve the separate ownership of semantic analysis, formatting, and governance implementation status.

**1.3.** An approved language rule is not itself evidence that every case is implemented. The 2026-10-09 missing-decisions entry reports that the ordinary whole-subtree `try` behavior is already implemented. Verify complete chained-call and mixed-error coverage through tests before updating implementation-status claims.

## 2. MD-028 — The complete protected expression

**2.1.** `try` protects the **entire following expression subtree**, through its natural grammatical boundary as already defined in `errorhandling.md` §13 and `runtime_checks.md` (“`try` boundaries”). This explicitly includes function-call receivers, call arguments, operators, indexing and every successive member/call in a method chain.

```sec
let result := try value.Fn1().Fn2().Fn3()
let item := try Build(left + right, values[index])
let output := try resource.Prepare().Transform(data[index]).Finish()
```

The snippets show expression forms, not declarations of `Fn1`, `Fn2`, `Fn3`, or any invented public API. Each expression is well-typed only if its actual called functions, members, arguments and ownership requirements are satisfied.

**2.2.** Each intermediate call is evaluated in normal Sec evaluation order. The successful value of a fallible intermediate operation is used to type-check and invoke the next member/call in the chain, where permitted by that member's type and ownership contract. The first encountered failure ends evaluation of the protected expression; no subsequent calls or operands are evaluated on that failure path.

**2.3.** Sema must resolve **all statically possible alternate/failure sources** in the protected subtree, including failures from call arguments and receiver expressions and language-defined checked operations. The resulting compiler-internal failure set is analysis data, **not** an anonymous source-level union, an invented public error type, or permission for automatic wrapping. It is not a runtime collection of errors: only the first encountered failure occurs along a given execution path.

**2.4.** For every possible failure path, Sema determines whether the failure:

1. is covered by a legal local `try` handler; or
2. can propagate directly through the enclosing function's declared compatible `Result[..., E]` or `Option[...]` return channel; or
3. cannot be handled or propagated as written, in which case compilation fails with a diagnostic identifying the incompatible failure source and a useful remedy.

Different concrete error types do **not** automatically require a local handler. In particular, a compatible explicitly declared `Result[..., error]` return channel can receive distinct concrete Sec error types through ordinary error widening, preserving concrete identity and payload. Conversely, the compiler must not infer `error`, a union, or a wrapper merely to make propagation or a heterogeneous local error binding succeed.

**2.5.** Local handlers are optional if all remaining failures can propagate. A handler is necessary where a failure cannot be propagated compatibly and the programmer must handle or map it explicitly. Existing `Err(_)`, typed `Err(...)`, guard, partial-handler, reachability, and ownership rules remain unchanged. A heterogeneous `Err(name)` catch-all still needs one compiler-resolved binding type under `errorhandling.md` §11.1; the complete-subtree rule does not relax this requirement.

**2.6.** `Option[T]`'s `None` alternate remains **absence**, not an error. `try` cannot implicitly map `None` to `Err(...)` or vice versa. The enclosing return channel or explicit local handling must satisfy the existing `Result`/`Option` propagation rules.

**2.7.** The body of a local handler and the handler's guard remain outside that same `try` expression's protected operation set. Fallible code in a guard or handler body requires its own handling where applicable. Parentheses may narrow the protected region according to the existing grammar.

**2.8.** `try` does not catch panic, manufacture error values for ordinary non-fallible operations, or alter evaluation order. Ownership, moves, borrows, destruction and cleanup remain those of the actual success/failure path.

**2.9. Documentation-only intent.** If the implementation already applies these rules to complete chains, receiver expressions, arguments, failure-set collection and compatibility checks, amend only the owning documentation and applicable governance status. If a focused test exposes a missing case, treat that as an implementation gap, **not** an invitation to invent alternative semantics.

## 3. MD-032 — Switch case-label indentation

**3.1.** Inside `switch`, each `case` or `default` label is indented **one configured indentation level** relative to the `switch` statement. The statements in that case body are indented **one further configured level** relative to its label. The closing `}` aligns with `switch`.

**3.2.** The number of spaces per indentation level is a **formatter/project option**, not an intrinsic language rule. Four spaces is the default only; another supported configured indentation width must produce the same structural hierarchy. This correction must not be integrated as “always four spaces.”

**3.3.** Canonical illustration with the **default four-space setting**:

```sec
switch value {
    case 1:
        HandleOne()
    case 2:
        HandleTwo()
    default:
        HandleOther()
}
```

**3.4.** Replace any owning-rulebook example that places `case`/`default` in the same column as `switch`. Do not change the currently implemented one-level/two-level indentation behavior merely to match an outdated example. This decision does not separately prescribe optional blank lines between cases.

## 4. MD-055 — Import-group headings and detached comments

**4.1.** A standalone group-heading comment within a grouped import declaration, when it is unambiguously attached to the immediately following imports, is placed **directly above the first import of its group**. There is **no blank line** between the heading comment and that import.

**4.2.** Separate distinct commented import groups with **one blank line**. The following is the canonical layout intended by the language owner, illustrated using the default four-space indentation setting:

```sec
import (
    // Network libraries
    "net/http"
    "net/url"

    // File libraries
    "fs"
    "io"
)
```

**4.3.** A group heading belongs to its group, **not** to an arbitrary individual import. Import sorting and regrouping must never move an import across a group boundary in a way that misrepresents the heading, nor attach a heading to the wrong import. Ordinary attached comments must travel with the source item to which they demonstrably belong, consistent with the existing formatter comment-attachment rules.

**4.4.** When comment attachment or group boundaries are **ambiguous**, the formatter must **preserve the order of the affected import region and the comments' relative positions**. It may apply unambiguous local whitespace/indentation changes but must not guess a comment's owner or invent a new grouping. In particular, a genuinely detached comment, trailing group comment, or inline block comment is not silently reclassified as a heading.

**4.5.** When a group-heading relationship is unambiguous, canonical formatting removes an unnecessary blank line **between that heading and its first import**. This specific layout correction does not authorize broader import reordering across comments or rewrite comment text.

**4.6.** The indentation width used inside `import (...)` is the same configured width as elsewhere. Four spaces in this example is a default, not a hardcoded language requirement. The formatter must be deterministic and idempotent for these groups.

## 5. Required regression and conformance coverage

**5.1. MD-028 — protected subtree:**

1. Test successive calls `try value.Fn1().Fn2().Fn3()` where separate stages are fallible; the first failure short-circuits later calls.
2. Test a failure in the receiver expression before method invocation.
3. Test an argument expression that fails before the containing call can execute.
4. Test checked arithmetic and indexing in arguments, including more than one concrete error type.
5. Test multiple distinct concrete errors with an explicit compatible `Result[..., error]` return: direct propagation is accepted and error identity is preserved.
6. Test a narrow declared error channel where one failure is incompatible: require explicit mapping/handling and report the offending path.
7. Test a heterogeneous `Err(name)` binding without an explicitly resolvable common channel: reject it according to §11.1; test `Err(_)` independently, including discardability.
8. Test parentheses narrowing and the natural grammatical end of the protected expression.
9. Test that guards and handler bodies are outside the same protected subtree.
10. Test that an `Option` `None` alternate is not implicitly transformed into a `Result` `Err` or vice versa.
11. Verify the appropriate resolved failure-set information is available to Semantic IR, diagnostics and LSP; keep downstream feature limitations separately tracked.

**5.2. MD-032 — switch formatting:**

1. Test `case`, `default` and body indentation under at least two supported configured indentation widths.
2. Verify closing-brace alignment and nested-switch indentation.
3. Verify formatter idempotence and no semantic changes.
4. Update golden fixtures/examples that follow the obsolete label column.

**5.3. MD-055 — grouped imports:**

1. Test the exact two-group example in §4.2, with **no empty line after either heading** and one empty line between groups.
2. Test normalization of an extra blank line after an unambiguously attached group heading.
3. Test that commented groups do not exchange imports across headings during sorting.
4. Test a detached comment, trailing comment after the last import, inline block comment and ambiguous attachment; preserve affected order and comment positions.
5. Test independently attached import comments and ensure any permitted movement retains their attachment.
6. Test alternative configured indentation widths, stable output and repeated formatter idempotence.

## 6. Integration and closing the missing-decision entries

**6.1.** Integrate MD-028 into the existing `try` expression, protected-check, internal failure-set, direct propagation and local-handler paragraphs of `rules/errors/errorhandling.md` and `rules/errors/runtime_checks.md`. Retain the old paragraph numbers and all references; add only minimal explicit clarifications and applicable tests.

**6.2.** Integrate MD-032 into the switch-formatting section and examples of `rules/tooling/formatter.md`. Make the text relative to the configured indentation width.

**6.3.** Integrate MD-055 into the imports and comment-attachment sections of `rules/tooling/formatter.md`. Preserve the distinct cases of an attached heading and a truly detached/ambiguous comment.

**6.4.** Synchronize the affected implementation/governance entries and verify the test evidence. Only then remove MD-028, MD-032 and MD-055 from the authoritative, most recently supplied `missing-decisions.yaml`, as required by its stated removal contract. Do **not** generate a replacement missing-decisions inventory or a separate audit as part of this correction.

**6.5.** This correction does not decide other open MD items, including error-pattern syntax (MD-029), MLIR try-handler representation (MD-031), import classification (MD-033), formatter manifest configuration (MD-036/MD-054) or unrelated binding rules.
