# MD-133 — Explicit returns and inferred result types in spawn blocks

- **Language version:** Sec 0.1 (retained in Sec 0.1.1; existing source grammar, resolved semantics)
- **Document revision:** 1.0
- **Date:** 2026-11-24
- **Status:** Owner-approved design; normative integration pending; implementation not verified
- **Decision:** MD-133 — `task-spawn-block-return-type`
- **Governance owner:** `concurrency.tasks-v2`; coordinate with `concurrency.spawn-v1` and the thread/process owners
- **Repository baseline:** `YoePro/sec`, main `3ed6048c0b9e25b28030a49c1f7cbe62421cd5e1`
- **Related decisions:** MD-128 (transactional spawn ownership), MD-129 (parent cancellation), MD-138 (callable execution requirements)
- **Primary rulebooks:** `rules/concurrency/spawn.md`, `rules/concurrency/tasks.md`, `rules/declarations/lambda-functions.md`
- **Related rulebooks:** `rules/concurrency/threads.md`, `rules/concurrency/processes.md`, `rules/declarations/functions.md`, `rules/foundations/grammar.md`, `rules/errors/errorhandling.md`, `rules/tooling/diagnostics.md`, `rules/compiler/semantic_ir.md`, and the ownership, destruction and transferability rulebooks.

## § 1. Approved decision

§ 1(1) All three concurrency creation forms shall follow the same source syntax and result-typing principle, except where the selected execution kind has an independently specified semantic restriction. This symmetry is intentional: programmers must not learn different rules for returning a value from a task, thread or process.

§ 1(2) Unqualified `spawn` and explicit `spawn task` remain semantically identical. The default execution kind is a logical task.

§ 1(3) For a spawned call or explicitly typed callable, `T` is its complete declared normal return type. For a spawn block, `T` is inferred from explicit return statements and ordinary control-flow analysis of the child body.

§ 1(4) A value leaves a spawn block as its normal result only through an explicit value-bearing `return`. A final expression statement, including a function call, is not an implicit return. A block with no value-bearing return has normal result type `void`, subject to all applicable control-flow, error-handling and ownership checks. A bare `return` is a void return.

§ 1(5) The earlier design discussion that would have treated a final unconsumed call as an implicit result is superseded by the owner's explicit decision on 2026-11-24. No tail-expression return rule is introduced.

§ 1(6) The one intentional asymmetry with ordinary named functions and lambdas is return-type inference for the existing spawn-block form. Ordinary function and lambda declarations still require an explicit return type and explicit value-bearing `return` statements. Spawn blocks do not create general block expressions or general inferred-return lambdas.

## § 2. Source forms and creation results

§ 2(1) Retain the existing `SpawnExpression` and `SpawnKind` productions in `grammar.md` § 89 and `spawn.md` § 2:

```ebnf
SpawnExpression
    ::= "spawn" [ SpawnKind ] Expression
      | "spawn" [ SpawnKind ] Block

SpawnKind
    ::= Contextual("task")
      | Contextual("thread")
      | Contextual("process")
```

§ 2(2) A spawn block denotes the body of a child callable with no declared source parameters or return annotation. It is a separate callable/control-flow boundary. Existing closure, capture, transferability and execution-context restrictions remain applicable. No new block annotation or implicit capture form is added.

§ 2(3) Every successful creation has an owning handle of the selected kind. The raw creation result and the successful payload are:

| Source form | Raw creation result | Successful payload |
|---|---|---|
| `spawn expression` or `spawn { ... }` | `Result[Task[T], TaskSpawnError]` | `Task[T]` |
| `spawn task expression` or `spawn task { ... }` | `Result[Task[T], TaskSpawnError]` | `Task[T]` |
| `spawn thread expression` or `spawn thread { ... }` | `Result[Thread[T], ThreadSpawnError]` | `Thread[T]` |
| `spawn process expression` or `spawn process { ... }` | `Result[Process[T], ProcessSpawnError]` | `Process[T]` |

§ 2(4) `try` around the spawn expression handles its creation result under existing error-handling rules. It does not alter the child's normal return type or handle its eventual returned `Result`, `Option`, cancellation, panic or execution failure. The selected kind's existing result transfer and lifecycle requirements apply.

## § 3. Return inference and ordinary control-flow validation

§ 3(1) The compiler shall distinguish return statements belonging to the spawn body from returns belonging to nested functions, lambdas and nested spawn bodies. Only the former contribute to the current body's return contract. An ordinary nested block, conditional, loop or match arm does not create a new callable boundary merely because it uses braces.

§ 3(2) Infer a single complete normal result type from the resolved value-bearing return expressions of the body, preserving nominal type identities and ordinary type and ownership rules. When no value-bearing return supplies a result type, infer `void`. Do not use the types of unused expression statements, local variable declarations, discarded results or values returned by nested callables as return candidates.

§ 3(3) Return expressions must resolve under ordinary Sec expression and literal-typing rules. All reachable normal return paths must satisfy the same resolved `T`; do not introduce implicit numeric conversion, common-supertype construction, `any` boxing, union synthesis, automatic `Result`/`Option` wrapping or a new overload-selection rule to make conflicting return expressions agree. Existing contextual shaping of literals may be applied against a uniquely established return type. If a unique complete type cannot be established, diagnose the unresolved return contract and suggest explicit typing of the returned value or an explicitly typed spawned lambda.

§ 3(4) After inference, apply the ordinary return and reachability checks in `functions.md` §§ 12–14 and `lambda-functions.md` §§ 5 and 35. A non-void body must produce a compatible explicit returned value on every reachable path that completes normally. Reachable fallthrough or a bare return on such a path is invalid. A void body may use a bare return or reach the end normally.

§ 3(5) A terminating path that does not complete normally, such as canonical `panic`, valid `cancel`, or proven divergence, does not supply a normal return value and does not require a fabricated one. Ordinary unreachable-code rules remain in force. A body that has no value-bearing return and never completes normally has `void` as its normal result type; this introduces no bottom type and does not imply that completion occurs.

§ 3(6) Return-type inference does not introduce general implicit-result expressions or permit returning an expression from a `void` body where the ordinary return rules forbid it. Multiple return values remain unsupported. A source expression of type `Result[V, E]`, `Option[V]`, a reference, a named type or a callable retains its complete resolved type, subject to the selected kind's result validity and transferability rules.

§ 3(7) The inference implementation may use deferred contextual checks and ordinary semantic resolution, but it shall not depend on source traversal order or optimization. A valid program must retain the same resolved result type under cold and incremental analysis.

### § 3A. Canonical examples

The examples below show the successful handle type after `try`. Assume a legal surrounding error-propagation context, supported execution kind and resolved handle lifecycle; these fragments are not complete programs.

```sec
fn Calculate() int {
    return 42
}

// Task[int]
let direct := try spawn Calculate()

// Task[int]
let explicit := try spawn {
    return Calculate()
}

// Task[void]; the final call result is discarded.
let ignored := try spawn {
    Calculate()
}

// Task[void]; the binding is not a return.
let stored := try spawn {
    let value := Calculate()
}

// Task[int]
let computed := try spawn {
    return 40 + 2
}

// Task[int]
let returned := try spawn {
    let value := Calculate()
    return value
}

// Task[void]
let noResult := try spawn {
    return
}

// Task[int]
let taskWorker := try spawn task {
    return Calculate()
}

// Thread[int]
let threadWorker := try spawn thread {
    return Calculate()
}

// Process[int], provided process result transfer is valid.
let processWorker := try spawn process {
    return Calculate()
}
```

## § 4. Informational diagnostic for an unused final call result

§ 4(1) A legal standalone call that returns a discardable non-void value retains the existing implicit-discard semantics. In a spawn block its position at the end of the body does not change this. In particular, a block inferred as `void` because it has no value-bearing return must not acquire a non-void result from such a call.

§ 4(2) When an otherwise valid void spawn block ends with an ordinary standalone call whose non-void result is implicitly discarded, the compiler shall report an informational diagnostic explaining that the missing explicit `return` leaves the successful handle type as `Task[void]`, `Thread[void]` or `Process[void]`, as appropriate. This is an advisory about valid source, not a mandatory error.

§ 4(3) Extend the existing registered `A3001` / `ownership.implicit-discarded-result` (`diagnostics.md` § 25(8)) with spawn-specific contextual explanation. Reuse its stable identity, default severity `Information`, configurable policy and deduplication rules. Do not emit a second independent advisory for the same implicitly discarded result merely to explain spawn inference. Report the call and its resolved result type and, where available, link to the selected declaration.

Example canonical English message and help, with the actual callable, type and kind substituted:

```text
The result of Calculate() (int) is discarded. This spawn block has no
value-bearing return and produces Task[void] on successful creation.

Help: Write 'return Calculate()' if the task should return the calculated
value, or explicitly discard the result if no value is intended.
```

§ 4(4) The successful handle type named by this message must not be confused with the raw creation result. Where the raw expression type is displayed, it remains `Result[Task[void], TaskSpawnError]` or its corresponding thread/process form.

§ 4(5) Do not emit this special explanation for a void call, explicit discard, a result consumed by an initializer or another expression, an explicit return, an unresolved or invalid call, or a block whose inferred result is non-void. Ordinary advisories for other discarded statements remain unaffected. A block with earlier value-bearing returns and invalid reachable fallthrough requires its ordinary mandatory return-path diagnostic, not an incorrect explanation that it returns `void`.

§ 4(6) Existing must-use and non-discardable-value rules take precedence. For example, an unused `Result` or unresolved owning execution handle cannot become valid by being the final expression statement in a spawn block. Disabling the informational diagnostic never makes an otherwise invalid discard legal. No new default or implicit handling of errors is introduced.

§ 4(7) Inserting `return` may intentionally change the handle type, lifecycle use, process-transfer requirements and later type checking. Offer this as programmer-directed help, not an automatic semantics-preserving rewrite. Do not suggest `return` as valid for a result whose return would violate the selected kind's requirements. Formatter and automatic fix-all operations shall never silently insert `return` or `discard`.

## § 5. Callable boundary, errors, and ownership

§ 5(1) A return in a spawn body exits the child callable, not the spawning function. A break or continue cannot cross the callable boundary. Defers and local cleanup belong to the child's invocation. A nested callable's return is not a return from its enclosing spawn body.

§ 5(2) A returned value follows ordinary return ownership, including transfer of an owning local without a mandatory `<-` at the return boundary. The result must survive child cleanup according to the existing return-value, borrow and destruction rules. No inferred type makes an otherwise invalid reference or transfer legal.

§ 5(3) Existing explicit-capture rules remain mandatory. A bare spawn block does not implicitly acquire enclosing local bindings. If a capture or an explicit result type is needed, use the existing explicit-capture/typed-callable form under the selected kind's ordinary transferability rules. Do not introduce a new capture clause for the bare block in this correction.

§ 5(4) Error propagation within a child belongs to that child's own result boundary. In particular, naked `try` inside the body must be checked against its own established compatible `Result` or `Option` return channel; the spawning function's result channel cannot satisfy this requirement. Inference must not automatically construct a result wrapper or infer a new error-union type merely from the presence of a `try`. If the ordinary rules cannot establish a complete compatible channel, require a typed returned value or an explicitly typed spawned lambda. This preserves `errorhandling.md` § 12 rather than introducing a new error-inference language.

§ 5(5) The selected kind's result ownership and transfer rules remain mandatory, including process result materialization. MD-128 governs prepared input ownership at creation and MD-129 governs task cancellation inheritance; neither is changed by this correction. The ordinary current-execution requirements, including the MD-138 amendment when integrated, apply to the actual child body, not to a fictitious parent invocation.

## § 6. Compiler, Semantic IR, and tooling

§ 6(1) Parsing retains the distinction between a spawn call/expression and a spawn block. Preserve return-statement and expression-statement nodes, source spans, explicit execution kind and ordinary nested control-flow structure. A final expression must not be rewritten to a return during parsing or recovery.

§ 6(2) Sema resolves the child callable boundary, all return candidates and the complete normal result type before forming the concrete handle type. Validate the selected kind, capture/transferability, effects, return paths, error channels and use of the raw creation result. Invalid or incomplete inference must not silently default an unresolved non-void return to `void`.

§ 6(3) Semantic IR shall record the selected execution kind, resolved child callable identity, complete normal `T`, raw creation result and appropriate handle identity, explicit return edges and distinct void fallthrough edges. Preserve normal result ownership, cleanup, effect requirements and the independently validated creation commit facts. An unused last expression is represented with ordinary evaluation/disposal semantics, not a synthetic return.

§ 6(4) Lowering consumes these verified facts. It shall not infer the result type from backend call results, choose a different type according to the execution kind, eliminate observable evaluation/cleanup of a discarded final call or fabricate a normal return payload for a terminal non-returning path.

§ 6(5) LSP hover and semantic queries show the inferred child return type and the correct raw/result-handle types. CLI, JSON and LSP preserve the advisory identity and configured severity. Formatter preserves explicit returns and does not change the presence or absence of returns. A known unimplemented valid feature remains an explicit compiler-capability limitation, not a contradictory source-language rule.

## § 7. Normative integration map

Integrate this correction into the owning books without renumbering existing paragraphs. Use new lettered sections or appended subparagraphs as appropriate to their existing numbering. Update headers, revision, date and correction provenance.

| Rulebook | Required synchronization |
|---|---|
| `concurrency/spawn.md` | §§ 2(2), 3(3), 16(1), 25–27: distinguish declared callable results from inferred block results, add explicit-return rule and informational diagnostic; preserve all three creation kinds. |
| `concurrency/tasks.md` | §§ 3(2), 4(2)–(3): include block-inferred `T` and raw creation type; retain ordinary outcome/lifecycle rules. |
| `concurrency/threads.md` | § 2(2)–(3): include inferred block result. Do not change deferred creation semantics. |
| `concurrency/processes.md` | §§ 7.1–7.2 and applicable callable/result-materialization rules: include inferred block result without relaxing process transfer requirements. |
| `declarations/lambda-functions.md` | §§ 3, 5, 35, 37: clarify the intentional exception for spawn-block return-type inference; ordinary lambda declaration/return rules unchanged. |
| `declarations/functions.md` | §§ 3, 12–14, 27: cross-reference the specific spawn-block exception; no ordinary function return inference. |
| `foundations/grammar.md` | §§ 47–48, 73, 89: cross-reference the result semantics; retain existing syntax. The separate stale process-feature statement in § 89(3) is not a new decision here; reconcile with the owning process rules in its appropriate synchronization. |
| `errors/errorhandling.md` | § 12: clarify the child result boundary after inference; preserve existing propagation and explicit typing rules. |
| `tooling/diagnostics.md` | § 25(8): extend A3001 with the spawn-specific explanatory context and tests; retain ID and policy. |
| `compiler/semantic_ir.md` | Synchronize the spawn/callable return facts with the owning semantic schema and verifier. |

The language feature was already admitted by the Sec 0.1 grammar. This correction resolves its undefined return semantics rather than adding a new syntax feature; it also applies to Sec 0.1.1. It must not change the version marker of the separate MD-137 or MD-138 syntax extensions.

## § 8. Conformance requirements

Tests shall distinguish source validity, frontend typing, required diagnostics, semantic-IR verification and executable runtime behavior. Test all applicable execution kinds on supported CompilationPlans. Where an implementation capability is incomplete, keep the relevant test/status pending; do not claim successful execution from parsing alone. For examples involving context-sensitive or must-use operations, use a complete valid fixture and resolve the resulting handle lifecycle.

| ID | Case | Required result |
|---|---|---|
| T01 | `spawn Calculate()` where `Calculate` returns `int` | Raw `Result[Task[int], TaskSpawnError]`. |
| T02 | Explicit `spawn task Calculate()` | Identical to T01. |
| T03 | `spawn thread Calculate()` | Raw `Result[Thread[int], ThreadSpawnError]`. |
| T04 | `spawn process Calculate()` | Raw `Result[Process[int], ProcessSpawnError]`, subject to process eligibility. |
| T05 | `spawn { return Calculate() }` | Raw task result with `T = int`. |
| T06 | Explicit task, thread and process block with `return Calculate()` | Same `T = int`, correct distinct handle/error identities. |
| T07 | `spawn { Calculate() }` | `T = void`; A3001 includes `Task[void]` explanation. |
| T08 | Same ignored final call in thread and process blocks | Correct void handle type and kind-specific advisory. |
| T09 | `spawn { let value := Calculate() }` | `T = void`; no missing-return advisory for the consumed call result. Other ordinary diagnostics remain applicable. |
| T10 | `spawn { return 40 + 2 }` | `T = int` under ordinary literal typing. |
| T11 | Bind a local and explicitly return it | `T` equals the complete resolved local type. |
| T12 | Empty block | `T = void`. |
| T13 | Bare `return` | `T = void`. |
| T14 | Final void call | `T = void`; no A3001. |
| T15 | Explicit discard of a legal non-void final call | `T = void`; no implicit-discard advisory. |
| T16 | Two reachable return branches with compatible resolved type | One complete inferred `T`. |
| T17 | Two incompatible resolved return types | Mandatory return-type diagnostic; no invented coercion or wrapper. |
| T18 | Reachable bare return and value-bearing return | Mandatory inconsistent normal-return diagnostic. |
| T19 | Value-bearing return on one path, reachable fallthrough on another | Mandatory missing-return-path diagnostic. |
| T20 | Value-bearing return on one path, valid terminating panic/cancel on another | No missing normal result on the terminating path. |
| T21 | Proven diverging body with no value-bearing return | `T = void`; no fabricated normal completion. |
| T22 | Return inside nested lambda or spawn | Does not determine outer spawn-body `T`. |
| T23 | Return inside ordinary nested conditional | Contributes to the enclosing spawn body's result contract. |
| T24 | Child return and enclosing function return | Distinct control-flow destinations and types. |
| T25 | Child returns an owning move-only local | Ordinary return transfer, cleanup and no double destruction. |
| T26 | Child attempts to return an invalid escaping reference | Existing mandatory lifetime/transfer diagnostic. |
| T27 | Child attempts implicit outer-local capture | Existing capture diagnostic; no new capture mechanism. |
| T28 | Final unused must-use `Result` | Mandatory existing diagnostic; no legal implicit return or discard. |
| T29 | Child explicitly returns a complete typed `Result[V, E]` | Nested result retained inside handle; no flattening. |
| T30 | Naked child `try` with established compatible child result channel | Existing propagation semantics within the child. |
| T31 | Naked child `try` with only the parent's compatible result channel | Rejected; no propagation across callable boundary. |
| T32 | Return type remains underconstrained (e.g. untyped `None` without usable context) | Diagnostic explains need for a resolvable type or typed callable. |
| T33 | Ordinary lambda omits explicit return type | Still invalid; no general inference. |
| T34 | Ordinary non-void lambda ends in a call without explicit return | Existing missing-return rule remains. |
| T35 | Advisory disabled/promoted by diagnostic policy | Standard A3001 policy; typing and mandatory discard rules unchanged. |
| T36 | Repeated semantic visits to same final call | One applicable A3001 occurrence per source statement. |
| T37 | Formatter round-trip | Return presence and execution kind unchanged. |
| T38 | Semantic IR for void block ending in discardable call | Call evaluated and disposed; no returned call result. |
| T39 | Semantic IR for block with explicit value return | Complete inferred T and valid return/cleanup edge preserved. |
| T40 | Process block returns non-ProcessTransferable value | Existing mandatory process-result validation. |
| T41 | Edit final call into explicit return in LSP | Handle type changes from void to resolved T; advisory removed; dependent diagnostics refreshed. |
| T42 | Failure of creation for a value-returning block | MD-128 ownership preserved; no child normal result and no successful handle. |

## § 9. Governance and implementation status

§ 9(1) Close MD-133 as an owner-approved design after integrating this normative correction; keep actual implementation status distinct. Link the applied correction and update the missing-decision description so it no longer describes the superseded implicit-void versus implicit-tail-expression choice as open.

§ 9(2) Synchronize existing governance owners, including `concurrency.tasks-v2` and `concurrency.spawn-v1`, with the corresponding thread/process, frontend, diagnostic, tooling and Semantic IR entries. Locate their owning fragments through `governance/index.yaml`; do not create duplicate feature identities. Track parser, Sema, result inference, diagnostic, IR/lowering, LSP/formatter and conformance evidence independently.

§ 9(3) Do not mark a track implemented merely because the existing parser accepts the block syntax. The baseline missing-decision entry reports a capability diagnostic for block spawn in the relevant implementation path. Replace that boundary only when the corresponding implementation and verification are complete.

§ 9(4) This document specifies the approved source contract and integration work; it does not claim that repository files, compiler implementation or governance have been modified.
