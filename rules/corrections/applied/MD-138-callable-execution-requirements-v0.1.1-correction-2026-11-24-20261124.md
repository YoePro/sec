# MD-138 — Callable execution requirements and the `require` clause

- **Language version:** Sec 0.1.1 (source-grammar extension; not retroactive to Sec 0.1)
- **Document revision:** 1.0
- **Date:** 2026-11-24
- **Status:** Owner-approved design and proposed normative integration; implementation not verified
- **Decision:** MD-138 — `cancellation-requirement-function-types`
- **Governance owner:** `frontend.cancellation-effect-v1`
- **Repository baseline:** `YoePro/sec`, main `3ed6048c0b9e25b28030a49c1f7cbe62421cd5e1`
- **Related decisions:** MD-052 (privileged low-level bodies), MD-053 (constraint effect guarantees), MD-128 and MD-129 (spawn and cancellation), MD-059 (unsafe callable types)
- **Primary rulebooks:** `rules/concurrency/cancellation.md`, `rules/declarations/lambda-functions.md`, `rules/declarations/functions.md`, `rules/foundations/grammar.md`, `rules/types/types.md`
- **Related rulebooks:** `rules/foundations/lexical_structure.md`, `rules/declarations/interfaces.md`, `rules/declarations/generics.md`, `rules/concurrency/spawn.md`, `rules/analysis/effect_analysis.md`, `rules/foundations/attributes.md`, `rules/compiler/semantic_ir.md`, `rules/compiler/monomorphization.md`, and the owning parser, formatter, diagnostics and LSP rulebooks.

## § 1. Purpose and scope

§ 1(1) Sec 0.1.1 introduces an explicit, statically verified callable invocation contract using the existing keyword `require`. The clause solves MD-138: a requirement inferred from a callable body must remain expressible and enforceable when that callable is passed, stored, returned or invoked indirectly.

§ 1(2) Automatic inference remains the default for body-bearing native Sec functions, methods and lambdas. Explicit declarations are available when a programmer wants to state a contract or when a type or bodyless callable signature must describe one. Programmers must not be required to repeat an inferred requirement at every ordinary call level.

§ 1(3) The clause describes conditions that the caller's execution environment must satisfy before invocation. It is not a parameter, ordinary value expression, runtime assertion, cancellation request, scheduler selection or function result.

§ 1(4) The initial requirements are exactly `task`, `thread` and `cancellable`. This correction establishes an extensible clause, but does not define arbitrary user predicates, effect variables, user-defined requirement names, requirement expressions, runtime checking, arbitrary Design by Contract, or additional requirement kinds.

§ 1(5) The existing distinction between task, thread and process execution remains intact. No execution kind acquires a new implicit context through this correction.

§ 1(6) The correction preserves Sec 0.1 behavior outside the stated amendment. In particular, ordinary return types, callable receiver capabilities, capture modes, ownership, borrow semantics, cancellation behavior and existing verified attributes remain governed by their owning rulebooks.

## § 2. Source syntax and lexical model

§ 2(1) A requirement clause appears after the complete callable return type. A body-bearing declaration places the clause before the body or its body-form introducer. An explicit function type places it after its return type. A bodyless native interface method signature uses the same placement.

```sec
fn() void
fn() void require(task)
fn() void require(thread)
fn() void require(cancellable)
fn() void require(task, thread)
```

```sec
fn InspectTask() void require(task) {
    let current := Task.Current()
    discard current.CancelRequested
}
```

```sec
let inspect := fn() void require(task) {
    let current := Task.Current()
    discard current.CancelRequested
}
```

§ 2(2) `require` remains an existing hard keyword. `task` and `thread` retain their existing contextual treatment, extended to requirement-entry positions. `cancellable` is recognized contextually in a requirement-entry position only; it is not a new globally reserved identifier or compiler-known value type. All spellings are case-sensitive.

§ 2(3) The requirement clause contains a nonempty, comma-separated sequence of entries. A trailing comma is legal, including in a multiline clause. Whitespace and comments follow ordinary Sec lexical rules. A clause may appear at most once per callable signature or function-type node.

§ 2(4) The entries form a conjunction: all listed requirements must hold. A list is not an ordered sequence of operations, and its ordering has no semantic significance. The same entry must not be repeated in a clause. Different entries that logically imply one another remain legal and are normalized semantically.

§ 2(5) A missing clause on an explicitly written native function type means no requirement from this requirement family. A missing clause on a bodyless native interface method has the same meaning. A missing clause on a body-bearing Sec function or lambda instead enables inference as defined in § 5. These meanings must not be conflated.

§ 2(6) `require()` is invalid. An unsupported entry, including `allocator`, `isr`, `process` or an arbitrary name, is diagnosed as unsupported in this language version. The compiler must not ignore an unknown entry or reinterpret it as an ordinary expression. Future versions may extend the entry grammar explicitly.

## § 3. Normative grammar delta

§ 3(1) Introduce the following productions in `grammar.md` under a new stable subsection adjacent to § 101, identified as a Sec 0.1.1 amendment. Preserve all existing section identifiers.

```text
ExecutionRequirementClause
    ::= "require" "(" ExecutionRequirement
        { "," ExecutionRequirement } [ "," ] ")"

ExecutionRequirement
    ::= Contextual("task")
      | Contextual("thread")
      | Contextual("cancellable")
```

§ 3(2) Extend `FunctionType` in `grammar.md` § 101:

```text
FunctionType
    ::= [ CallableCapability ] "fn"
        "(" [ CallableParameterType
              { "," CallableParameterType } [ "," ] ] ")"
        TypeReference
        [ ExecutionRequirementClause ]
```

The existing productions for `CallableCapability` and `CallableParameterType` remain unchanged.

§ 3(3) Extend native Sec declaration and signature productions in `grammar.md` § 32:

```text
FunctionDeclaration
    ::= "fn" Identifier [ GenericParameterList ]
        ParameterList TypeReference
        [ ExecutionRequirementClause ]
        FunctionBody

FunctionSignature
    ::= "fn" Identifier [ GenericParameterList ]
        ParameterList TypeReference
        [ ExecutionRequirementClause ]
```

The body remains mandatory for ordinary definitions. For interface methods in § 27, extend the native `MethodSignature` tail, after its return `TypeReference`, with the same optional clause. Keep all existing receiver-capability and static-member restrictions. This does not legalize consuming interface receivers, bodyless ordinary prototypes or other previously invalid declarations.

§ 3(4) Extend `LambdaExpression` in `grammar.md` § 73:

```text
LambdaExpression
    ::= CaptureClause? "fn" ParameterList TypeReference
        [ ExecutionRequirementClause ] Block
```

The existing explicit parameter/return type and capture rules remain unchanged. This amendment is not a change to the spawn-block return rules in MD-133.

§ 3(5) A requirement clause is a suffix of a callable signature, not a suffix of every `TypeReference`. It is not an attribute and cannot be placed after an arbitrary scalar, struct or collection type.

§ 3(6) Nested callable return types retain the existing right-recursive `TypeReference` structure. A suffix attaches to the innermost immediately preceding unparenthesized callable signature that can consume it. Parenthesizing a returned function type explicitly closes that inner type before the enclosing signature's clause.

```sec
// The returned callable requires a task.
fn() fn() void require(task)

// The outer callable requires a task.
fn() (fn() void) require(task)

// The outer callable requires a thread; its result requires a task.
fn() (fn() void require(task)) require(thread)
```

The same binding rule applies to a named function or lambda returning a callable. The parser must not guess ownership of a clause from line breaks, indentation or semantic context. An outer clause must follow a parenthesized returned callable type when the unparenthesized return type would otherwise consume that clause. A second suffix cannot be used to bypass the one-clause rule.

```sec
fn CreateCallback() (fn() void) require(task) {
    return fn() void {}
}
```

§ 3(7) The clause precedes the function-body form. MD-052 remains the owner of any privileged `mlir` body grammar. For compatibility with the design under discussion, the intended composition is:

```text
fn CoreFunction() int require(thread) mlir { ... }
```

This is an illustrative composition, not approval of an MLIR-body production, additional body kind or backend contract. An implementation must not accept `mlir` through MD-138 alone. Low-level bodies must eventually preserve the same callable contract using the verification mechanisms established by their own rulebook.

§ 3(8) This amendment does not extend the dedicated foreign-function grammar or `c::fn` types, nor introduce new syntax for property accessors, lifecycle members, module declarations or attributes. Any execution-context requirements of those constructs remain subject to their existing rules; explicit syntax for those positions, if needed, is a separate amendment.

## § 4. Exact requirement semantics

§ 4(1) The initial vocabulary and satisfaction conditions are:

| Source entry | Semantic meaning | Satisfaction condition |
|---|---|---|
| `task` | Requires current logical task | A real current `TaskContext` exists under the task model. |
| `thread` | Requires current physical thread context | The active target/profile and execution environment establish a valid physical/native thread context as required by `Thread.Current()`. |
| `cancellable` | Requires current cancellable execution | Bare `cancel` has an eligible current logical task or, in the absence of a logical task, an eligible explicit cancellable physical thread as defined by `cancellation.md` § 14. |

§ 4(2) `task` implies `cancellable`. `thread` does not imply `cancellable`: ordinary main or an attached foreign thread may provide physical context without an eligible cancellable execution identity. `cancellable` does not imply `task` or `thread`; a logical task may run on a target without the relevant physical-thread capability. `task` does not imply `thread`, and `thread` does not imply `task`.

§ 4(3) These are requirements on invocation, not classifications of code as asynchronous or parallel. No clause creates a task, starts a thread, creates a process, injects an implicit `Context`, requests cancellation, sets `CancelRequested`, or selects a scheduler. It does not grant ownership or cancellation-request authority over a handle.

§ 4(4) `require(cancellable)` states availability of an eligible execution identity. It does not promise that a cancellation request is present or that invocation will result in cancellation. The meaning of bare `cancel`, precedence of logical tasks over physical threads, request observation and cleanup remain unchanged.

§ 4(5) Requirement satisfaction is established statically using canonical execution-context and `CompilationPlan` facts. Missing proof is not permission. A failed proof is a compilation diagnostic, not a runtime `Err`, panic, implicit spawn, or silently disabled operation. Runtime values and target-runtime observation cannot satisfy this static contract by themselves.

§ 4(6) Availability of an execution identity is distinct from permission to perform a particular operation at a particular point. Existing restrictions for ISR, `defer`, shielding, FFI, unsafe code and other restricted contexts remain independently enforced. This clause cannot waive them or create a new cancellation point.

## § 5. Inference and explicit contracts

§ 5(1) Every concrete native callable has a semantic execution-requirement summary, separate from its ordinary signature and its may-effect/guarantee summaries. The summary is part of the callable's resolved semantic type and invocation contract.

§ 5(2) Infer requirements from the owning language rules, including:

| Operation | Requirement contribution |
|---|---|
| `Task.Current()` or `Task.Yield()` | `task` |
| `Thread.Current()` | `thread` |
| Bare `cancel` | `cancellable` |
| Ordinary synchronous call | Callee's effective requirements |
| Indirect synchronous invocation | Resolved function type's effective requirements |

This table does not redefine any other operation's effects. Existing context-aware operations that may also execute outside a cancellable entity are not made context-dependent merely because they may respond to cancellation when such an entity exists.

§ 5(3) Infer the conjunction of the requirements of all semantically reachable operations and ordinary calls, preserving the established reachability rules. Mutually recursive callables require a stable fixed point. Generic specialization uses the concrete semantic types, callees and requirement contracts, and must preserve the existing generic-contract verification rules. Inference is not permitted to depend on optional optimization or a lucky runtime target.

§ 5(4) If a native callable body has no explicit clause, its effective requirement contract is the inferred summary. This applies to ordinary functions, methods and lambdas. The LSP may render the inferred contract using the same spelling as an explicit clause, but must distinguish inferred information from actual source text.

```sec
fn NeedsTask() void {
    let current := Task.Current()
    if current.CancelRequested {
        cancel
    }
}

let callback := NeedsTask
```

Here the effective contract is `require(task)`. The inferred type of `callback` is `fn() void require(task)`; the redundant cancellable requirement is implied by the task requirement.

§ 5(5) An explicit clause on a native body-bearing callable is its declared invocation contract. The compiler must prove that every execution environment satisfying the declaration also satisfies the requirements inferred from the implementation. A stronger declaration than the implementation currently needs is legal and remains part of its public contract. The compiler must not silently strengthen an insufficient explicit declaration or weaken an intentionally stronger one.

```sec
// Valid: the explicit contract is sufficient.
fn Inspect() void require(task) {
    let current := Task.Current()
    discard current.CancelRequested
}

// Valid: the declared contract is stronger than the body needs.
fn ReservedForTasks() void require(task) {
}

// Invalid: a cancellable explicit thread does not establish a task.
fn Invalid() void require(cancellable) {
    let current := Task.Current()
    discard current.CancelRequested
}
```

§ 5(6) An insufficient contract is diagnosed at the declaration with the first relevant originating operation/callee and the missing requirement. A compiler must not accept the body simply because all currently known callers happen to satisfy a stronger condition.

§ 5(7) Requirement inference does not cross a callable-definition boundary merely because a closure is constructed, captured, stored, returned or moved. The contract belongs to the callable being constructed. Evaluation of capture expressions, arguments and other operations in the current execution context retains its own ordinary requirements. Invoking the callable, including indirect invocation, is checked separately.

§ 5(8) An operation that is independently prohibited in a context remains prohibited; deriving a stronger callable requirement is not a means to legalize an invalid operation. At a concrete entry/root or any boundary with a fixed execution contract, the available context must satisfy the requirements. A context-free program entry must not be accepted by inventing an inferred root task.

## § 6. Canonical type identity and compatibility

§ 6(1) The canonical function type includes its normalized execution-requirement contract in addition to all existing callable capability, parameter, ownership, borrow, variadic and return type components. The contract is not determined by parameter names, capture contents, textual entry order or whether the requirement was inferred or written explicitly.

§ 6(2) Normalize the initial vocabulary as a set of semantic requirements. If `task` is present, `cancellable` is implied and redundant in the canonical identity. The original source clause remains available for diagnostics and formatting. For example, `require(thread, task)` and `require(task, thread, cancellable)` have equal requirement identities, without requiring their source spellings to be rewritten.

§ 6(3) For a callable assignment or conversion, let `S` be the source callable's effective requirements and `D` the destination function type's declared requirements. The requirement component is compatible exactly when the execution environments permitted by `D` are a subset of those permitted by `S`:

```text
D implies S
```

The destination may restrict valid invocation further, but it must never promise invocation in a context in which the source callable cannot execute. This is independent of, and additional to, all existing callable receiver-capability and signature checks.

| Source requirements | Destination requirements | Compatibility |
|---|---|---|
| none | none | valid |
| none | task | valid |
| task | none | invalid |
| cancellable | task | valid |
| task | cancellable | invalid |
| thread | task | invalid |
| thread | task, thread | valid |
| task, thread | task | invalid |
| task, cancellable | task | valid; equivalent |

§ 6(4) Compatibility is determined from the portable callable contract, not from incidental facts at the assignment site. For example, assignment of a task-requiring function to plain `fn() void` is invalid even when executed inside a task. The destination might escape and be invoked elsewhere. Known target identity does not waive this rule.

```sec
let inferred := NeedsTask
let typed: fn() void require(task) := NeedsTask
let invalid: fn() void := NeedsTask
```

The first two bindings are valid. The third is invalid. This explicitly closes the closed-target annotation loophole noted in MD-138.

§ 6(5) Once a callable value is converted to a stronger explicit invocation contract, invocation uses that destination contract. Known implementation facts may support optimization but cannot silently weaken the declared contract. No new conversion that strips requirements is introduced.

§ 6(6) Native callable requirements remain attached across legal storage in structs, unions, collections, generic arguments, return values and other typed storage. Ordinary ownership, copy/move, capture and lifetime rules remain in force. No type-erasure or generic facility may be used to erase a required invocation condition; where an existing facility cannot preserve the required contract it must reject the incompatible operation rather than treating the value as context-free.

§ 6(7) The requirement dimension composes with ordinary callable capabilities:

```sec
mut fn() int require(task)
-> fn(-> Resource) Handle require(thread)
```

A valid execution context does not grant mutable or consuming access to the callable environment. A valid environment capability does not establish the execution context.

§ 6(8) Ordinary overload selection continues to use its existing parameter-type and compatibility rules, now including callable requirement identity where a parameter itself is a callable type. This correction does not introduce overloading of otherwise identical function declarations solely by their own `require` suffix, runtime dispatch by execution context, or a new overload-ranking preference based on the current context.

## § 7. Indirect calls, interfaces and generics

§ 7(1) Calls through an explicitly typed parameter must use its declared invocation contract regardless of whether the callee's concrete address or target set is known. No whole-program target discovery is required for soundness.

```sec
fn Execute(callback: fn() void require(task)) void {
    callback()
}
```

`Execute` infers `require(task)`. It can be invoked from a valid task context, but not directly from ordinary program entry without that context.

§ 7(2) A function that merely transports a task-requiring callback need not require a task itself:

```sec
fn Forward(
    callback: fn() void require(task)
) fn() void require(task) {
    return callback
}
```

The requirement shown here belongs to the returned callable, not to `Forward` itself. `Forward` performs no invocation. Any independent requirements from its implementation must still be inferred.

§ 7(3) Statically resolved generic instantiations preserve the callable requirement components of their substituted types. No general effect variable, effect-polymorphic `fn` type or implicit existential requirement is introduced. An explicitly written plain `fn(...) T` retains its context-free invocation contract for this family of requirements even when it appears inside a generic declaration.

§ 7(4) A native interface method may declare an execution requirement using the same suffix. An omitted clause on a bodyless requirement means no requirement from this family. A concrete conforming method must have an effective contract compatible with that of the interface method under § 6(3). An interface dispatch or generic constraint call uses the declared interface member contract, including its requirements.

```sec
interface TaskWorker {
    fn Work() void require(task)
}
```

A compatible method may require a task, require only a cancellable execution context, or require neither, provided every other member contract is satisfied. A method requiring a physical thread in addition to a task cannot satisfy this interface's contract.

§ 7(5) Existing interface inheritance and conflicting-member rules remain unchanged; normalized requirement identity participates in semantic signature comparison. The implementation may not merge conflicting inherited contracts by inventing a weaker or stronger combined signature.

§ 7(6) This interface amendment concerns invocation requirements only. MD-053 continues to own the unresolved syntax and verification of effect guarantees such as `@noAlloc`, `@noPanic` and `@noBlock` on interface requirements. It is not closed or otherwise resolved by MD-138.

## § 8. Spawn and execution boundaries

§ 8(1) Ordinary synchronous calls inherit the current invocation context. A spawn boundary is different: the selected callable is invoked in a new execution identity. Its execution requirements must be checked against that new identity and the active target/profile, not blindly propagated to the creator.

§ 8(2) `spawn` and `spawn task` establish a real logical task context and satisfy `task` and `cancellable` for the child callable. A `thread` requirement additionally needs proof of a valid physical thread context for the child's permitted execution environments. `spawn thread` establishes a physical thread and eligible explicit cancellable thread execution under the thread rules; it satisfies `thread` and `cancellable` but does not implicitly establish `task`.

§ 8(3) `spawn process` establishes a separate process isolation domain. It does not automatically provide a current logical task or eligible cancellable thread for the process callable. A physical-thread requirement must be proved from the actual child bootstrap and target contracts, never from the creator's thread. Process materialization and transferability checks remain mandatory. This amendment introduces no new process cancellation semantics or implicit child execution identities.

§ 8(4) Evaluation of spawn arguments and callable/capture preparation occurs in the creator's context where the spawn rules require it. Their requirements remain requirements of that evaluation. A child contract cannot satisfy a requirement of a creator-side expression. When a spawned block has its own callable body, requirements of operations in the block belong to that child invocation.

§ 8(5) The same rules apply to eager and deferred thread creation. No deferred start may escape validation of the eventual child's execution requirements. No creation-failure error variant is introduced for missing static context proof.

§ 8(6) MD-128 ownership commit, MD-129 cancellation inheritance and detachment, and MD-133 spawn-block results remain independent decisions. `require` does not change handle types, result types, ownership transfer, scheduling, task parentage or cancellation propagation.

## § 9. Extensibility and separation of contract categories

§ 9(1) A function's contract distinguishes:

- Invocation requirements: conditions the caller/execution environment must satisfy (`require(...)`).
- Verified guarantees: facts the callable promises to uphold, including existing attributes such as `@noAlloc`, `@noPanic` and `@noBlock`.
- Implementation/body form: the representation of the function body, including a future privileged low-level body only if its owning decision permits it.

§ 9(2) Do not reinterpret existing verified attributes as requirements. `require(cancellable)` is not `@noPanic`, does not establish any `@noBlock` guarantee and does not guarantee that the callable reaches a cancellation point. Requirements and effects are orthogonal semantic facts.

§ 9(3) Existing `MayAllocate` and `RequiresAllocationContext` are separate facts (MLIR Package 19 § 52). A future allocation-context requirement is a plausible extension of this clause; no `require(allocator)` spelling or allocation contract is approved by MD-138. Existing ambient allocation context propagation, explicit Arena use and provider selection are unchanged.

§ 9(4) Future additions may define parameterized entries, target/platform requirements or additional invocation conditions by extending the owned grammar and providing exact satisfaction, inference, normalization, compatibility and diagnostic rules. They must not be accepted merely because the clause is designed to be extensible.

§ 9(5) In particular, `require(y != 0)` and `require(feature(...))` are not legal in this revision. General argument-value preconditions and arbitrary Design by Contract are outside this decision; the syntax extension does not prejudge whether they should later be expressed using `require` or another construct.

## § 10. Compiler, IR and tooling obligations

§ 10(1) The lexer retains existing keyword behavior. The parser recognizes requirement clauses in the exact approved positions and preserves their attachment, individual entries, source spans, delimiters and original spelling. Malformed or unknown entries require focused recovery without consuming a following body or declaration unnecessarily.

§ 10(2) Sema stores canonical requirements in callable type identity, validates explicit contracts, computes inferred requirements over relevant call edges, verifies conversions and checks actual invocation boundaries. Missing or unresolved facts must not be represented as a proven empty requirement set.

§ 10(3) Function/closure type construction, overload resolution, interface conformance, generic substitution, monomorphization, cross-module exported signatures, callable storage and indirect dispatch must consume the same canonical facts. A signature with changed requirements is a semantic dependency change for incremental analysis and cache compatibility. Optional optimization or analysis-depth settings must not disable mandatory checking.

§ 10(4) Semantic IR must preserve the requirements of callable values, their invocation contracts, relevant call-site context proofs and spawn-boundary validation. The verifier rejects a call/assignment that would erase or fail to satisfy the contract. Existing ownership, cancellation, context ABI and lowering contracts remain intact. No runtime requirement mask, extra source parameter, particular hidden ABI parameter or dynamic assertion mechanism is mandated by this correction.

§ 10(5) LSP hover and signature help show declared or inferred requirements, distinguishing them from other effects. Completion within a requirement clause offers the three supported entries and does not advertise future entries. Diagnostics identify the missing or incompatible requirement and a useful correction. For inferred requirements, provide the originating operation or callee chain when available.

§ 10(6) Formatter output preserves clause attachment, explicit presence, source entry order and meaning. It formats the clause as `require(task, thread)` with ordinary comma spacing; multiline formatting follows the existing list conventions. It must not add an inferred clause, remove an explicitly declared stronger clause, delete a redundant entry, or drop a parenthesis necessary for attachment. Formatting does not act as a semantic rewrite.

§ 10(7) Illustrative diagnostics (not new assigned diagnostic IDs):

```text
callable requires a current logical task; destination fn() void does not preserve that requirement
help: use fn() void require(task) as the callable type
```

```text
function Invalid declares require(cancellable), but Task.Current() requires a current logical task
help: declare require(task) or remove the task-dependent operation
```

```text
cannot invoke this callable in the current execution context: a logical task is required
```

```text
unknown execution requirement 'allocator' in Sec 0.1.1
help: supported execution requirements are task, thread, and cancellable
```

Existing diagnostics registration and compiler-as-mentor conventions govern the actual diagnostic identifiers and presentation.

## § 11. Required conformance coverage

§ 11(1) Test syntax and parser/formatter round trips, semantic compatibility, context validation, effect inference, IR preservation, diagnostics and incremental invalidation independently. The following are normative test scenarios; they are not claims about existing implementation support.

| ID | Scenario | Expected outcome |
|---|---|---|
| T01 | Plain `fn() void` type and ordinary body-bearing function with no requirements | Existing source remains valid. |
| T02 | Each of `require(task)`, `require(thread)`, `require(cancellable)` on native function type, function definition and lambda | Parses and resolves. |
| T03 | Multiple entries, trailing comma, multiline clause, comments | Valid; correct attachment retained. |
| T04 | Empty clause, repeated entry, repeated clause on the same signature | Focused diagnostic. |
| T05 | `require(allocator)`, `require(process)`, arbitrary identifier, literal, boolean expression or nested parameterized entry | Invalid in v0.1.1. |
| T06 | `cancellable` as an ordinary local identifier outside the clause | Valid under ordinary name rules. |
| T07 | `require` after an ordinary non-callable type | Invalid. |
| T08 | Nested callable return type with suffix on inner callable | Inner requirement only. |
| T09 | Parenthesized callable return type followed by outer suffix | Outer requirement only. |
| T10 | Parenthesized inner callable with its own clause and outer clause | Both attach correctly. |
| T11 | Function using `Task.Current()` and/or `Task.Yield()` | Infers `task`. |
| T12 | Bare `cancel` alone | Infers `cancellable`; no invented task requirement. |
| T13 | `Thread.Current()` alone | Infers `thread`; no invented task/cancellable requirement. |
| T14 | `Task.Current()` plus `cancel` | Canonical requirement `task`. |
| T15 | Inferred ordinary calls, recursion and concrete generic substitution | Preserves transitive contract. |
| T16 | Independent explicit stronger clause on a body that needs no context | Retained contract; call still checked. |
| T17 | `require(cancellable)` body invokes `Task.Current()` | Reject insufficient explicit contract. |
| T18 | Inferred binding to task-dependent named function or lambda | Requirement preserved in type. |
| T19 | Explicit plain `fn() void` initialized from task-requiring callable, including known closed target and assignment within a task | Reject erasure. |
| T20 | Explicit `fn() void require(task)` initialized from context-free callable | Valid; destination invocation contract remains. |
| T21 | Assignment compatibility matrix in § 6(3) | All outcomes match; unrelated type/mode mismatches still reject. |
| T22 | Invoke task-requiring callback parameter in wrapper | Wrapper infers task requirement even with open target set. |
| T23 | Invoke wrapper directly at ordinary context-free program entry | Reject missing context. |
| T24 | Invoke same wrapper in an established task | Valid with respect to this requirement family. |
| T25 | Store or return task-dependent callable without invoking it | Transport does not itself inherit invocation requirement. |
| T26 | Struct/union/collection/generic storage with compatible callable type | Requirements survive legal storage and retrieval. |
| T27 | Same storage through incompatible plain callable type | Reject erasure. |
| T28 | Interface contract with task requirement and context-free/cancellable/task-dependent conforming methods | Accept if all other contracts match. |
| T29 | Context-free interface method contract implemented by task-requiring method; task contract implemented by task+thread method | Reject. |
| T30 | Interface or generic constraint invocation | Propagates declared requirement, including across open dispatch. |
| T31 | `spawn` or `spawn task` with task-requiring callable from ordinary main | Child requirement satisfied; no parent task fabricated. |
| T32 | `spawn thread` with task-requiring callable | Reject missing logical task. |
| T33 | `spawn thread` with cancellable/thread-requiring callable on eligible target | Valid subject to thread rules. |
| T34 | Task spawn requiring thread on profile without valid physical thread context | Reject; no implicit physical context. |
| T35 | Process spawn with task-requiring callable and no proven child task context | Reject. |
| T36 | Task-requiring function called in creator-side spawn argument from context-free creator | Reject; child's context cannot satisfy creator-side operation. |
| T37 | Context requirement combined with `mut fn`, `-> fn`, ownership/borrow modes or capture | Each contract dimension checked independently. |
| T38 | `require(cancellable)` used to attempt independently prohibited cancellation operation | Existing prohibition still enforced. |
| T39 | Missing context proof with optional folding disabled or lower LSP analysis depth | Mandatory semantic rejection unchanged. |
| T40 | Requirements differ only by order or implied cancellable entry | Equal canonical requirement identity; preserve source spelling. |
| T41 | Low-level `mlir` body or foreign function type made valid solely by this correction | Reject/not enabled by MD-138. |
| T42 | Edit a callee to introduce/remove a requirement; analyze dependents incrementally | No stale callable type, signature or context proof. |
| T43 | IR indirect call attempts to erase a requirement or invoke without necessary proof | Verifier rejects. |
| T44 | Existing verified attributes combined with a valid requirement clause | Independent guarantees remain checked; no grant of new attribute targets. |
| T45 | Call in ordinary context-free entry with actual physical thread support, but without eligible cancellable execution | `thread` may be satisfied; `task` and `cancellable` are not. |

§ 11(2) Test fixtures must use the actual owning target/profile and legal lifecycle resolution. Successful spawn examples must handle creation results and resolve the resulting handles. Rejection fixtures must isolate the relevant missing requirement rather than accidentally relying on unrelated syntax, ownership or lifecycle errors.

## § 12. Normative integration and governance

§ 12(1) Integrate this correction into the owning rulebooks without renumbering existing paragraphs. The following references identify the integration locations in the repository baseline, not a mandate to overwrite unrelated recent revisions.

| Owner | Integration |
|---|---|
| `foundations/grammar.md` §§ 27, 32, 73, 99–101, 115 | Clause production, approved native signatures, nested binding, v0.1.1 version boundary. |
| `foundations/lexical_structure.md` §§ 7.1–7.2 | Existing hard keyword and contextual entry handling. |
| `concurrency/cancellation.md` §§ 19–24, 56–62 | Requirements, declared/inferred semantics, indirect calls, satisfaction and diagnostics. |
| `declarations/lambda-functions.md` §§ 7–12, 35–42 | Callable type identity, compatibility, body inference, storage and AST obligations. |
| `declarations/functions.md` §§ 2–4, 19–25 | Native signature suffix, ordinary call and overload behavior. |
| `types/types.md` § 33 | Normalized requirement contract in callable types. |
| `declarations/interfaces.md` §§ 3–6 | Bodyless native method contract and conformance. |
| `declarations/generics.md`, `compiler/monomorphization.md` | Preserve callable requirement identity and constraint-call contract through substitution. |
| `concurrency/spawn.md` §§ 7–10, 16, 25–26 | Child-context validation distinct from creator-side evaluation. |
| `analysis/effect_analysis.md`, `compiler/semantic_ir.md` | Distinguish requirements from may-effects; preserve proofs and invocation contracts. |
| Owning parser, formatter, LSP and diagnostic rulebooks | Synchronize recognition, rendering, source attachment, diagnostics and tests. |

§ 12(2) Record MD-138 as owner-decided after normative integration according to the actual repository decision-ledger workflow. A completed design is not proof of implementation. Update the governance fragment indexed under `frontend.cancellation-effect-v1` and cross-link affected owners; do not duplicate competing ownership entries or mark implementation complete before verified tests and applicable lowering paths exist.

§ 12(3) Implement in separately verified steps: grammar/AST and formatter; semantic requirement identity and compatibility; direct/indirect inference and root validation; interfaces/generics and spawn boundaries; Semantic IR/lowering, diagnostics and LSP; conformance/incremental tests. Test each step before proceeding. Preserve existing source semantics and diagnostics outside the amendment.

§ 12(4) Do not resolve MD-052, MD-053, MD-059 or new allocation-context syntax in this integration. Existing owners retain their respective decisions. No compiler implementation or repository edit is claimed by this document.

---

### Integration notes (non-normative)

The approved design fixes the clause spelling and placement, its initial three requirement kinds, the inferred/explicit contract distinction, callable compatibility, and future extensibility. Detailed mechanical choices in this revision include rejecting duplicate entries, canonicalizing logically implied requirements, preserving source spelling in the formatter, and applying the same native signature contract to bodyless interface methods. The latter is necessary to avoid recreating MD-138 at interface dispatch boundaries. These choices are made explicit for owner review rather than silently introduced into the existing rulebooks.

The extension deliberately does not assign a public spelling to ambient allocator, ISR, process or target-feature requirements. It does not specify a general argument precondition language. If a future revision needs such a requirement, its owning decision must define its satisfaction relation and propagation before the parser accepts it.
