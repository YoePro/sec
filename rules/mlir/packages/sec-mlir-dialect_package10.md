# Sec MLIR Program - Implementation Package 10

## Package status

Implementation package for the Sec compiler.

Package ID: `SEC-MLIR-P10`\
Package title: `Local Try Handlers and Result Branching`\
Repository: `https://github.com/YoePro/sec`\
Repository branch: `main`\
Repository sync commit used for this package: `152c772`\
Repository sync date: `2026-08-09`\
Required Semantic IR version: `1`\
Required Sec MLIR dialect schema: `5`\
Sec MLIR dialect schema after package: `6`\
Sec MLIR lowering specification after package: `6`

Package 10 implements local `try` handler control flow for the error type already
available through Package 9:

```text
ArithmeticError
```

It also introduces the canonical high-level Result branch/unwrapping model
required for `try` over ordinary `Result[T, ArithmeticError]` values.

This package does not introduce physical Result layout and does not introduce
general user enum/union lowering.

---

# 1. Normative authority

Implementation follows:

```text
rules/errors/errorhandling.md
rules/errors/runtime_checks.md
rules/foundations/operators.md
rules/errors/panic.md
rules/library/core-library.md
rules/analysis/effect_analysis.md
    ↓
rules/compiler/semantic_ir.txt
    ↓
rules/mlir/sec_mlir.md
    ↓
rules/mlir/sec_mlir_dialect.md
    ↓
rules/mlir/sec_mlir_lowering.md
    ↓
implementation package
    ↓
implementation
```

Before implementation:

1. update `rules/mlir/sec_mlir_dialect.md` with
   `sec_mlir_dialect_package10.md`;
2. update `rules/mlir/sec_mlir_lowering.md` with
   `sec_mlir_lowering_package10.md`.

No source-language grammar change is required.

---

# 2. Current frontend reality

At repository sync `152c772`, the AST already contains:

```text
TryExpression
    Expression
    Handlers []*TryHandler

TryHandler
    Pattern
    Body
    ReturnBody
    BlockBody
```

Sema already implements:

```text
top-to-bottom handler checking
first matching handler
Err(identifier) catch-all binding
Err(enum variant) specific matching
duplicate/unreachable handler detection
enum-error exhaustiveness
fallback-expression type checking
return-handler checking
handler-local binding scope
```

Package 10 must reuse and expose these resolved facts.

Do not create a second source-level handler resolver inside Semantic IR.

---

# 3. Normative try-handler rules implemented

For:

```sec
let value := try operation() {
    Err(pattern) => handler
}
```

the canonical semantics are:

```text
Ok(value):
    continue with the success value unless an explicit Ok handler replaces the
    implicit success path

Err(error):
    inspect handlers from top to bottom
    select the first matching handler
```

Handler forms:

```text
expression fallback
return statement
block
```

A value-context handler must:

```text
produce a fallback value assignable to T
or return from the enclosing function
or propagate/return Err(...)
or terminate control flow
```

The handler block must be exhaustive.

Unmatched errors are not automatically propagated when a handler block exists.

---

# 4. Package 10 error-type scope

The first complete Semantic IR/Sec MLIR implementation supports local handlers
for:

```text
ArithmeticError
```

from Package 9.

Supported error patterns:

```sec
Err(ArithmeticError.Overflow)
Err(ArithmeticError.DivisionByZero)
Err(ArithmeticError.InvalidShift)
Err(error)
```

where:

```text
Err(error)
```

is a catch-all binding of the complete `ArithmeticError` value.

Supported success patterns:

```sec
Ok(value)
Ok(_)
```

The implicit success path remains the normal default.

---

# 5. Deliberate error-type boundary

Do not implement arbitrary user-defined error enum/union lowering in Package 10.

Examples deferred from the new Semantic IR path:

```sec
Err(IOError.InvalidValue)
Err(ParseError.InvalidNumber)
Err(InvoiceError.Calculation)
```

unless that error type already has a canonical Semantic IR/Sec MLIR value
representation from another completed package.

Reason:

Package 10 must not invent an error-only representation for user enum/union
types that would later conflict with the canonical enum/union representation.

This is an implementation boundary, not a language restriction.

---

# 6. Wide builtin invariant

These remain active Sec builtin types:

```text
int128
int256
uint128
uint256
decimal128
```

Package 10 fallback/result success values may use the active wide integer types.

No future/planned wording is permitted.

---

# 7. Package goal

After Package 10:

1. Sema exposes a stable read-only resolved try-handler plan;
2. token-scanning heuristics are not used by the new IR builder to determine
   handler flow;
3. Semantic IR represents Result discrimination explicitly;
4. Semantic IR represents safe Ok/Err unwrapping after a branch;
5. naked propagation of ordinary `Result[T, ArithmeticError]` is implemented;
6. local handlers for `Result[T, ArithmeticError]` are implemented;
7. local handlers for fallible arithmetic from Package 9 are implemented;
8. implicit Ok success is implemented;
9. explicit `Ok(value)` and `Ok(_)` handlers are implemented;
10. specific ArithmeticError variant handlers are implemented;
11. catch-all `Err(error)` is implemented;
12. top-to-bottom first-match behavior is preserved;
13. exhaustiveness is preserved;
14. unreachable handlers remain frontend errors;
15. fallback expressions merge through SSA block parameters;
16. return/Err handlers terminate their handler path correctly;
17. no unmatched error is auto-propagated when handlers exist;
18. Result physical layout remains unresolved;
19. user enum/union error representation remains deferred;
20. generated Sec MLIR has a verifiable canonical Result guard shape.

---

# 8. Package boundary

## 8.1 In scope

Implement:

```text
resolved try-handler Sema API
structural handler flow classification
Semantic IR ResultIsErrOp
Semantic IR ResultUnwrapOkOp
Semantic IR ResultUnwrapErrOp
Semantic IR local try handler plan
Semantic IR CoreErrorIsVariantOp
implicit success handling
explicit Ok(value)
explicit Ok(_)
Err(ArithmeticError.Overflow)
Err(ArithmeticError.DivisionByZero)
Err(ArithmeticError.InvalidShift)
Err(error) catch-all binding
ordered handler dispatch
exhaustive variant-only handlers
fallback value merge
return handler
Err propagation/return handler
terminating block handler
ordinary Result naked propagation for ArithmeticError
local handlers on ordinary Result values
local handlers on Package 9 arithmetic failures
Sec MLIR schema version 6
sec.result.is_err
sec.result.unwrap_ok
sec.result.unwrap_err
sec.core_error.is_variant
Result guard verifier
handler dispatch verifier
Package 8/9 compatibility
end-to-end tests
full regression
```

## 8.2 Explicitly out of scope

Do not implement:

```text
physical Result layout
Result tag width
Result payload union layout
LLVM Result struct

general enum representation
general union representation
user-defined error enum lowering
user-defined error union lowering

automatic error mapping
automatic union wrapper selection
implicit error conversion

try assignment
fallible property setter handler lowering

general match lowering
general enum matching
general Result match

richer patterns
guards on try handlers

ownership-sensitive handler payload moves
borrow patterns

defer cleanup integration beyond existing return/termination semantics

panic endpoint lowering
LLVM dialect
production backend migration
```

---

# 9. Sema resolved handler API

Add a read-only resolved plan keyed by `*ast.TryExpression`.

Recommended types:

```go
type ResolvedTryHandlerPatternKind string

const (
    TryHandlerImplicitOk  ResolvedTryHandlerPatternKind = "implicit-ok"
    TryHandlerOkBinding   ResolvedTryHandlerPatternKind = "ok-binding"
    TryHandlerOkDiscard   ResolvedTryHandlerPatternKind = "ok-discard"
    TryHandlerErrVariant  ResolvedTryHandlerPatternKind = "err-variant"
    TryHandlerErrCatchAll ResolvedTryHandlerPatternKind = "err-catch-all"
)

type ResolvedTryHandlerFlow string

const (
    TryHandlerProducesValue ResolvedTryHandlerFlow = "produces-value"
    TryHandlerReturns       ResolvedTryHandlerFlow = "returns"
    TryHandlerTerminates    ResolvedTryHandlerFlow = "terminates"
)
```

Recommended plan:

```go
type ResolvedTryHandler struct {
    PatternKind ResolvedTryHandlerPatternKind
    Variant     string
    BindingName string
    BindingType Type
    Flow        ResolvedTryHandlerFlow
    ResultType  Type
    SourceIndex int
}

type ResolvedTryPlan struct {
    SuccessType   Type
    ErrorType     Type
    HasExplicitOk bool
    Exhaustive    bool
    Handlers      []ResolvedTryHandler
}
```

Exact names may differ.

---

# 10. Read-only query

Recommended API:

```go
func (a *Analyzer) ResolvedTryPlanOf(
    expr *ast.TryExpression,
) (ResolvedTryPlan, bool)
```

The query:

```text
returns facts recorded during successful Sema
does not infer expression types
does not resolve patterns
does not check exhaustiveness
does not mutate Analyzer
```

Semantic IR lowering consumes the plan.

---

# 11. Replace token-presence flow heuristics

Current frontend code contains a helper equivalent to:

```text
blockContainsReturn
```

that identifies `return` by scanning block tokens.

Package 10 must not expose that heuristic as the semantic truth consumed by
Semantic IR.

The rulebook requires structural control-flow analysis capable of distinguishing:

```text
continues normally
returns
produces a value
propagates an error
terminates
```

For Package 10 handler plans, Sema must provide a structurally valid resolved
flow classification.

At minimum, do not accept:

```sec
Err(error) => {
    if condition {
        return Err(error)
    }
}
```

as a valid value-context terminating handler merely because a `return` token
appears somewhere in the block.

---

# 12. Sema remains authoritative

Semantic IR must not determine:

```text
whether a pattern is a variant or binding
whether a handler is unreachable
whether handlers are exhaustive
whether a fallback type is assignable to T
whether an Ok handler is explicit
whether an Err handler binds a value
handler source order
```

All of those come from `ResolvedTryPlan`.

---

# 13. Semantic IR Result discrimination

Add operations:

```text
ResultIsErrOp
ResultUnwrapOkOp
ResultUnwrapErrOp
```

They operate on:

```text
Result[T,E]
```

No physical Result representation is implied.

---

# 14. `ResultIsErrOp`

Input:

```text
result: Result[T,E]
```

Output:

```text
isErr: bool
```

Total semantic operation.

Meaning:

```text
false -> Result is Ok
true  -> Result is Err
```

No memory effect.

---

# 15. `ResultUnwrapOkOp`

Input:

```text
Result[T,E]
```

Output:

```text
T
```

Semantic precondition:

```text
the control-flow path is proven to be the Ok path of the same Result value
```

For `T=void`, no value-producing unwrap is required.

This is an internal payload projection validated by the Result guard verifier.

---

# 16. `ResultUnwrapErrOp`

Input:

```text
Result[T,E]
```

Output:

```text
E
```

Semantic precondition:

```text
the control-flow path is proven to be the Err path of the same Result value
```

Validated by the Result guard verifier.

---

# 17. Canonical Result guard CFG

For one Result SSA value `%result`:

```text
%isErr = result.is_err %result
cond_br %isErr, errBlock, okBlock

errBlock:
    %error = result.unwrap_err %result
    ...

okBlock:
    %value = result.unwrap_ok %result
    ...
```

No unwrap occurs before the branch.

No Ok unwrap is valid on the Err path.

No Err unwrap is valid on the Ok path.

---

# 18. Result guard verifier

Add:

```bash
--sec-verify-result-guards
```

It is a verification pass.

For compiler-generated canonical Result branching it checks:

```text
ResultIsErr result has exactly one branch use
the branch condition is ResultIsErr result
true successor is the Err block
false successor is the Ok block
Err block unwraps Err from the same Result SSA value
Ok block unwraps Ok from the same Result SSA value when T != void
unwrap occurs before payload use
no opposite unwrap exists in either canonical branch
Result SSA value dominates both branches
```

---

# 19. Ordinary Result naked propagation

Package 10 generalizes naked propagation for already-representable:

```text
Result[T, ArithmeticError]
```

Source:

```sec
let value := try Operation()
```

inside:

```text
Result[U, ArithmeticError]
```

Canonical:

```text
ResultIsErr
cond_br

Err branch:
    unwrap Err
    construct ResultErr[U, ArithmeticError]
    return

Ok branch:
    unwrap Ok T
    continue
```

No local handler dispatch exists on naked propagation.

---

# 20. Local Result handler entry

For:

```sec
let value := try Operation() {
    ...
}
```

where:

```text
Operation() : Result[T, ArithmeticError]
```

first perform canonical Result guard.

Then:

```text
Ok branch -> implicit or explicit Ok handling
Err branch -> ordered local Err handler dispatch
```

No automatic propagation of unmatched Err is permitted.

---

# 21. Arithmetic local handler entry

For:

```sec
let value := try left / right {
    ...
}
```

Package 9/P8 already provides:

```text
success value
failed
reason
```

Canonical P10:

```text
failed false:
    implicit or explicit Ok handling with arithmetic result

failed true:
    convert reason -> ArithmeticError
    ordered local Err handler dispatch
```

Do not construct a temporary Result merely to branch it again.

---

# 22. Unified local handler abstraction

After source-specific success/error extraction, both:

```text
Result expression
checked arithmetic expression
```

enter the same conceptual handler engine:

```text
success: T
error: ArithmeticError

success -> implicit/explicit Ok path
error   -> ordered Err handlers
```

This unification belongs in Semantic IR builder logic, not Sema.

Sema has already resolved handler meaning.

---

# 23. ArithmeticError variant testing

Add Semantic IR:

```text
CoreErrorIsVariantOp
```

Input:

```text
core ArithmeticError
```

Attribute:

```text
variant
```

Allowed P10 variants:

```text
Overflow
DivisionByZero
InvalidShift
```

Output:

```text
bool
```

No memory effect.

---

# 24. Ordered Err dispatch

Specific handlers are tested in source order.

Example:

```sec
Err(ArithmeticError.DivisionByZero) => first
Err(ArithmeticError.Overflow) => second
Err(error) => third
```

Canonical CFG:

```text
test DivisionByZero
    true -> first
    false -> test Overflow

test Overflow
    true -> second
    false -> catch-all third
```

Do not sort handlers by enum declaration order.

Do not reorder handlers for optimization in the builder.

---

# 25. Catch-all binding

For:

```sec
Err(error) => ...
```

the handler block receives/binds:

```text
the complete exact ArithmeticError SSA value
```

The binding:

```text
is immutable
has exact ArithmeticError type
has handler-local scope
```

No copy/move semantics beyond the current trivial core-error value model are
introduced.

---

# 26. Exhaustive variant-only dispatch

A handler block may be exhaustive without a catch-all:

```sec
Err(ArithmeticError.Overflow) => ...
Err(ArithmeticError.DivisionByZero) => ...
Err(ArithmeticError.InvalidShift) => ...
```

Sema has already proven coverage.

Canonical lowering may:

```text
compare the first two in source order
route the final unmatched value to the sole remaining variant handler
```

provided the verifier proves:

```text
the resolved handler plan covers all three variants
the final handler is the only remaining possible variant
```

This avoids adding an impossible unmatched runtime path.

---

# 27. Catch-all unreachable rule

Sema rejects:

```sec
Err(error) => ...
Err(ArithmeticError.Overflow) => ...
```

as unreachable.

Semantic IR never receives this as a valid resolved handler list.

No downstream pass must compensate for it.

---

# 28. Specific duplicate rule

Duplicate specific error handlers are rejected by Sema or classified as
unreachable.

Package 10 assumes no duplicate live variant handler.

---

# 29. Implicit Ok path

When no explicit Ok handler exists:

```text
success value T
    -> normal try-expression success continuation
```

For value context, the success continuation passes T to the merge.

For statement-like context where the value is unused but legally consumed, no
value merge is required.

---

# 30. Explicit Ok binding

For:

```sec
Ok(value) => handler
```

the handler-local binding receives:

```text
success T
```

The binding is immutable.

The implicit success behavior is replaced by the explicit Ok handler.

Only one live explicit Ok handler is valid.

---

# 31. Explicit Ok discard

For:

```sec
Ok(_) => handler
```

the success value is not bound.

Existing underscore/discard semantics remain context-specific.

Package 10 does not redefine global underscore rules.

---

# 32. Fallback expression

For value context:

```sec
Err(ArithmeticError.DivisionByZero) => 0
```

Sema has already proven the expression assignable to T.

Semantic IR:

```text
evaluate fallback expression
branch to try merge with fallback T
```

The implicit/explicit Ok success value also branches to the same merge.

Merge block:

```text
one block parameter of T
```

The `try` expression result is that block parameter.

---

# 33. Return handler

Example:

```sec
Err(error) => return Err(error)
```

Canonical:

```text
handler error binding
ResultErr construction for enclosing declared Result
func return
```

No branch to the try merge.

The exact enclosing error type rules are already resolved by Sema.

For Package 10 this path supports exact `ArithmeticError`.

---

# 34. Early plain return

A handler may return another value valid for the enclosing function.

Lower using normal Semantic IR return semantics.

No try merge edge.

---

# 35. Terminating handler block

A handler block proven by Sema to terminate:

```text
has no edge to the try merge
```

Package 10 does not infer termination by scanning AST tokens.

It consumes the resolved handler flow.

---

# 36. No implicit unmatched propagation

With local handlers:

```sec
try operation() {
    Err(...) => ...
}
```

an unmatched error must not be silently propagated.

Sema requires exhaustiveness.

Semantic IR verifier requires the resolved handler plan to be exhaustive.

If downstream IR somehow lacks an exhaustive target:

```text
compiler bug
```

Do not synthesize `return Err(error)`.

---

# 37. Error mapping boundary

Package 10 allows explicit mapping only when the handler body itself constructs
a type already representable in Semantic IR.

It does not choose wrappers.

Package 10 does not implement arbitrary user error unions just to enable
mapping.

---

# 38. General user error enum boundary

Although current Sema supports enum error patterns such as:

```sec
Err(IOError.InvalidValue)
```

the new Semantic IR path may report an explicit unsupported-feature error until
general enum value representation is implemented.

The parser/Sema feature remains valid.

Do not downgrade the source-language rule.

---

# 39. General Result error-type boundary

P10 Result branching operations are generic over:

```text
Result[T,E]
```

at the dialect/type level.

Compiler generation is required in P10 only when E is already canonically
representable.

Required end-to-end E:

```text
ArithmeticError
```

---

# 40. Semantic IR verifier

Required checks:

```text
ResultIsErr operand is Result
ResultIsErr result is bool
ResultUnwrapOk output equals Result success type
ResultUnwrapErr output equals Result error type
CoreErrorIsVariant operand is exact core ArithmeticError in P10
variant is one of three ArithmeticError variants
try handler plan is marked exhaustive
no duplicate live variant handlers
at most one explicit Ok handler
fallback merge values exactly match T
returning handlers have no merge successor
terminating handlers have no merge successor
catch-all is final live Err handler
```

Cross-block Result guard validation belongs to the dedicated verifier.

---

# 41. Sec MLIR dialect schema version 6

Compiler-generated high-level Sec MLIR now uses:

```mlir
sec.dialect_version = 6 : i32
```

Schema versions 1-5 remain regression inputs.

Schema 6 adds:

```text
sec.result.is_err
sec.result.unwrap_ok
sec.result.unwrap_err
sec.core_error.is_variant
```

---

# 42. `sec.result.is_err`

Operand:

```text
!sec.result<T,E>
```

Result:

```text
i1
```

Total.

No memory effect.

No physical tag layout implied.

---

# 43. `sec.result.unwrap_ok`

Operand:

```text
!sec.result<T,E>
```

Result:

```text
T
```

For non-void T.

Valid only in a Result guard success block.

---

# 44. `sec.result.unwrap_err`

Operand:

```text
!sec.result<T,E>
```

Result:

```text
E
```

Valid only in a Result guard error block.

---

# 45. `sec.core_error.is_variant`

Operand:

```text
!sec.core_error<"core::ArithmeticError">
```

Allowed variants:

```text
Overflow
DivisionByZero
InvalidShift
```

Result:

```text
i1
```

Total.

No physical enum representation implied.

---

# 46. Result guard verifier in MLIR

Register:

```bash
--sec-verify-result-guards
```

Required checks:

```text
same Result SSA for test and projection
true successor is Err
false successor is Ok
Err projection occurs only on Err path
Ok projection occurs only on Ok path
Result dominates projections
```

---

# 47. Try handler verifier in MLIR

Register:

```bash
--sec-verify-try-handlers
```

Recommended handler provenance:

```text
sec.try_handler_kind
sec.try_handler_index
sec.try_handler_variant
```

Kinds:

```text
ok
err-variant
err-catch-all
merge
```

Verifier checks ordering, catch-all finality, exhaustive ArithmeticError
coverage, merge typing, and explicit/implicit Ok exclusivity.

---

# 48. P8/P9 compatibility

P9 arithmetic local handlers use:

```text
reason -> ArithmeticError -> handler dispatch
```

P8 may replace the checked integer operation with standard Arith while
preserving:

```text
failed
reason
failure CFG
```

P10 never re-analyzes arithmetic.

---

# 49. Effect analysis

A locally handled arithmetic failure contributes no arithmetic panic effect when
every failure path is handled without reaching `sec.fail.arithmetic`.

Handler-body effects remain.

`try` does not catch unrelated panics.

---

# 50. Result ownership boundary

End-to-end P10 support is limited to Result payloads compatible with the current
Semantic IR ownership subset.

If T or E requires unimplemented move/copy/destruction semantics:

```text
UnsupportedFeatureError
```

No silent payload copying.

---

# 51. Required tests

Sema:

```text
ordered handlers
catch-all unreachable
duplicate variant
exhaustive ArithmeticError variants
non-exhaustive rejected
fallback exact type
Ok binding/discard
binding scope
structural conditional-return validation
```

Semantic IR:

```text
Result guard operations
naked Result propagation
implicit Ok
explicit Ok
specific variant chain
catch-all
fallback merge
return/terminate no merge
arithmetic local handler
Result call local handler
wide success values
```

MLIR:

```text
new operations parse/print/verify
canonical Result guard accepted
invalid Result projections rejected
ordered handler metadata accepted
invalid handler order/coverage rejected
schema-v5 regression accepted
```

---

# 52. End-to-end examples

Required:

```text
int32 arithmetic fallback
int64 division variant handlers
int128 arithmetic catch-all
uint256 shift InvalidShift handler
explicit Ok handler
return Err(error) handler
Result[int32, ArithmeticError] local handlers
Result[int128, ArithmeticError] naked propagation
Result[uint256, ArithmeticError] local handlers
```

No hand editing of generated IR.

---

# 53. Unsupported tests

Explicitly reject in the new Semantic IR path:

```text
local handler on unsupported user error enum
local handler on unsupported user error union
try assignment
handler requiring non-trivial ownership movement
automatic error mapping
general Result match
```

These are implementation boundaries, not language errors.

---

# 54. No physical Result/error lowering

Package 10 still does not define:

```text
Result discriminant
Result payload layout
core-error integer encoding
ABI aggregate representation
LLVM representation
```

---

# 55. No mandatory runtime

Local handlers use:

```text
SSA
branches
variant tests
fallback values
returns
Result construction
```

No exception runtime, unwinder, heap allocation, or mandatory Sec runtime.

---

# 56. Acceptance criteria

Package 10 is complete only when:

```text
[ ] repository baseline 152c772 or newer sync documented
[ ] previous package regressions remain green
[ ] wide-builtin invariant remains
[ ] schema v6 rulebook installed
[ ] lowering v6 rulebook installed
[ ] Sema resolved try plan API exists
[ ] handler flow is structural
[ ] Semantic IR Result guard ops exist
[ ] Semantic IR CoreErrorIsVariant exists
[ ] naked Result[*,ArithmeticError] propagation works
[ ] local Result handlers work
[ ] local arithmetic handlers work
[ ] implicit Ok works
[ ] explicit Ok(value) works
[ ] explicit Ok(_) works
[ ] specific ArithmeticError handlers work
[ ] catch-all binding works
[ ] handler order preserved
[ ] variant-only exhaustive handlers work
[ ] fallback SSA merge works
[ ] return/terminate paths do not merge
[ ] no unmatched automatic propagation
[ ] schema-v6 dialect ops implemented
[ ] result guard verifier registered
[ ] try handler verifier registered
[ ] P8/P9 compatibility passes
[ ] wide handler tests pass
[ ] unsupported user errors fail explicitly
[ ] no physical Result/error layout selected
[ ] no mandatory runtime
[ ] check-sec-mlir passes
[ ] go test ./... passes
[ ] legacy paths remain operational
```

---

# 57. Required implementation report

Codex must report:

```text
1. repository HEAD implemented against
2. previous package status
3. files added
4. files modified
5. Sema resolved try-plan API
6. replacement of token-based handler-flow heuristic
7. Semantic IR Result guard operations
8. Semantic IR handler CFG algorithm
9. implicit/explicit Ok handling
10. ordered Err dispatch algorithm
11. catch-all implementation
12. exhaustive variant-only implementation
13. fallback merge strategy
14. return/termination strategy
15. schema-v6 operations
16. Result guard verifier
17. try handler verifier
18. P8/P9 compatibility changes
19. wide integer handler tests
20. ordinary Result propagation tests
21. unsupported error-type cases
22. CMake commands
23. exact LLVM/MLIR version
24. check-sec-mlir result
25. go test ./... result
26. end-to-end local handler results
27. deviations
28. recommendations for Package 11
```

---

# 58. Package 11 boundary

Recommended Package 11:

```text
Enum and Union Semantic Value Representation
```

This is the natural next step because P10 deliberately stops before inventing
special representations for user-defined error enums/unions.

Recommended scope:

```text
Semantic IR enum types
Semantic IR enum values
enum underlying type identity
enum variant identity
active integer underlying widths
Semantic IR union types
union variant identity
payload/no-payload union variants
Sec MLIR enum type/value operations
Sec MLIR union type/value operations
high-level variant tests
no physical LLVM layout yet
```

Once Package 11 exists, the P10 handler engine can be extended cleanly from:

```text
ArithmeticError
```

to user-defined error enums/unions without a parallel error-specific type system.
