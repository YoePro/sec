# Sec MLIR Program - Implementation Package 12

## Package status

Implementation package for the Sec compiler.

Package ID: `SEC-MLIR-P12`\
Package title: `Enum, Union, Result and Option Match CFG`\
Repository: `https://github.com/YoePro/sec`\
Repository branch: `main`\
Repository sync commit used for this package: `152c772`\
Repository sync date: `2026-08-09`\
Semantic IR version before package: `1`\
Semantic IR version after package: `1`\
Sec MLIR dialect schema before package: `7`\
Sec MLIR dialect schema after package: `8`\
Sec MLIR lowering specification before package: `7`\
Sec MLIR lowering specification after package: `8`

Package 12 lowers resolved Sec `match` semantics into explicit Semantic IR and
Sec MLIR control flow.

The implementation covers the canonical variant-oriented match domain:

```text
enum
union
Result[T,E]
Option[T]
catch-all
where guards
match statements
match expressions
```

The match subject is evaluated exactly once.

Arms are considered in source order.

The first arm whose pattern and optional guard both succeed is selected.

General structural destructuring, literal/range matching, ownership-sensitive
payload extraction and physical enum/union lowering remain outside this package.

---

# 1. Normative authority

Implementation follows:

```text
rules/flowcontrol_match.txt
rules/enums.txt
rules/unions.txt
rules/errorhandling.txt
rules/copy_move.md
rules/borrowing.txt
rules/ownership.md
    ↓
rules/semantic_ir.txt
    ↓
rules/sec_mlir.md
    ↓
rules/sec_mlir_dialect.md
    ↓
rules/sec_mlir_lowering.md
    ↓
implementation package
    ↓
implementation
```

Before implementation:

1. apply `sec_match_enum_domain_package12.md` to the normative match/enum rules;
2. apply `sec_semantic_ir_match_package12.md` to `rules/semantic_ir.txt`;
3. update `rules/sec_mlir_dialect.md` with
   `sec_mlir_dialect_package12.md`;
4. update `rules/sec_mlir_lowering.md` with
   `sec_mlir_lowering_package12.md`.

The enum-domain amendment is required before coding enum match lowering.

---

# 2. Repository state at baseline

At repository sync `152c772`:

```text
AST already has MatchExpression, MatchStatement and MatchArm
parser already supports expression/statement match
parser already supports catch-all _
parser already supports payload bindings
parser already supports where guards
Sema already analyzes Result patterns
Sema already analyzes Option patterns through the compiler-known union model
Sema already analyzes enum patterns
Sema already analyzes union variants
Sema already supports positional union payload bindings
Sema already supports ref/ref mut payload bindings
Sema already checks guards are bool
Sema already checks enum/union/Result exhaustiveness
Sema already detects duplicate/unreachable arms
Sema already merges definite-assignment/move/borrow state
Sema already infers match-expression result type
```

Package 12 must reuse those resolved frontend semantics.

The Semantic IR builder must not run a second match type checker.

---

# 3. Wide builtin invariant

These are active Sec builtin types:

```text
int128
int256
uint128
uint256
decimal128
```

Package 12 must not describe any of them as future, planned, reserved,
placeholder or not-yet-active.

Match subjects, payloads and expression results may use the active wide integer
types wherever the existing Semantic IR value subset supports them.

---

# 4. Required enum-match semantic correction

Current enum and match rules contain a semantic inconsistency:

```text
enum aliases may have the same numeric value
explicit integer-to-enum conversion may produce a value with no declared case
enum runtime representation is the underlying integer value
current match exhaustiveness counts declared case names
```

Those facts cannot all be implemented literally.

Package 12 resolves this in favor of the already-defined enum value semantics.

Canonical rule:

```text
an enum pattern matches the semantic numeric enum value
```

not hidden declaration provenance.

---

# 5. Enum alias pattern equivalence

For:

```sec
enum ResultCode int {
    Success = 0
    Ok = 0
}
```

patterns:

```text
ResultCode.Success
ResultCode.Ok
```

match the same runtime value set.

Therefore:

```sec
match code {
    ResultCode.Success => A()
    ResultCode.Ok => B()
    _ => C()
}
```

has an unreachable second arm.

The compiler must diagnose it.

A guarded alias may precede an unguarded alias/value-equivalent pattern:

```sec
match code {
    ResultCode.Success where condition => A()
    ResultCode.Ok => B()
    _ => C()
}
```

because the guarded arm does not fully cover numeric value `0`.

---

# 6. Enum exhaustiveness domain

An enum's runtime value domain is the complete set of values representable by
its underlying enum representation.

Reason:

```text
explicit integer-to-enum conversion may create an enum value with no declared
case name
```

Therefore declared case names alone are not generally exhaustive.

Example:

```sec
enum Direction {
    North
    East
    South
    West
}
```

A match containing only those four patterns is not exhaustive if other
underlying integer values can exist.

Use:

```sec
match direction {
    Direction.North => North()
    Direction.East => East()
    Direction.South => South()
    Direction.West => West()
    _ => Other()
}
```

unless the compiler proves that the declared unguarded numeric patterns cover
the complete representable enum domain.

---

# 7. Finite full enum-domain proof

The compiler may prove an enum match exhaustive without `_` when the unique
unguarded numeric pattern set covers the complete representable domain.

Examples where this is realistic:

```text
bit[1]
bit[2]
small fixed-width integer enum with all values declared
```

Coverage is by unique numeric value, not declaration name.

Use arbitrary-precision range/cardinality logic.

Do not enumerate the domain when a range/cardinality proof is sufficient.

If representation width/range is not known at the analysis point, require `_`
rather than guessing.

---

# 8. Enum diagnostic changes

Recommended diagnostics:

```text
unreachable enum match arm; numeric value is already covered by <earlier case>
```

and:

```text
non-exhaustive match for <Enum>; enum may contain undeclared numeric values;
add _ or cover the complete underlying domain
```

The exact diagnostic IDs follow the repository diagnostic registry.

Do not weaken ordinary duplicate/unreachable diagnostics.

---

# 9. Package goal

After Package 12:

1. Sema records a stable read-only `ResolvedMatchPlan`;
2. enum aliases are normalized to numeric coverage classes for matching;
3. enum exhaustiveness reflects the real underlying value domain;
4. Semantic IR evaluates every match subject exactly once;
5. Semantic IR emits source-order pattern test CFG;
6. guards execute only after their pattern matches;
7. guard failure continues with the next arm;
8. no later pattern/guard executes after one arm body is selected;
9. enum match uses P11 enum operations;
10. union match uses P11 union guard/projection operations;
11. Result match uses P10 Result guard/projection operations;
12. Option match uses the canonical union model;
13. catch-all `_` works;
14. match expressions merge values through SSA block parameters;
15. returning/terminating arms do not branch to the merge;
16. match statements merge only continuing control flow;
17. exhaustive residual fallthrough is represented as synthesized
    `UnreachableOp`;
18. Sec MLIR uses standard `cf` for match CFG;
19. schema v8 adds only the general `sec.unreachable` terminator plus reserved
    match-provenance metadata;
20. `--sec-verify-match-cfg` verifies compiler-generated match chains;
21. by-value single payload binding is implemented for copy-trivial payloads;
22. payload discard `_` is implemented without extraction;
23. ownership-sensitive payload binding remains explicit unsupported;
24. struct-like whole-payload binding remains explicit unsupported until
    canonical struct Semantic IR exists;
25. no general match runtime or dispatch table is introduced.

---

# 10. In-scope source match forms

Required end-to-end source domains:

```text
fieldless enum patterns
enum catch-all
enum where guards

payload-less union variants
single-payload union variants
single-payload copy-trivial binding
single-payload discard binding _
union catch-all
union where guards

Result Ok(value)
Result Ok(_)
Result Err(error)
Result where guards

Option Some(value)
Option Some(_)
Option None
Option where guards

match statement
match expression with expression arms
return arm
supported terminating arm
```

---

# 11. Explicitly deferred source forms

Do not implement in the new Semantic IR path in P12:

```text
union ref payload binding
union ref mut payload binding
Result ref/ref mut payload binding
Option ref/ref mut payload binding

move-only by-value payload binding
semantic-copy payload binding
conditional-copy payload binding
non-copyable payload binding

struct-like union whole-payload binding
direct struct field destructuring
nested patterns
pattern alternatives

literal patterns
range patterns
runtime type patterns
general bool match lowering

loop continue/break from match arms until loop Semantic IR exists
ownership-sensitive discard/destruction caused by an ignored arm value
```

These are implementation boundaries, not source-language removals.

---

# 12. Struct-like union binding boundary

Current source semantics permit:

```sec
Circle(circle) => Process(circle.Radius)
```

for a struct-like union payload.

The current Sema models the binding as a synthetic struct type.

Package 12 does not invent a temporary match-only struct representation.

Until canonical struct Semantic IR exists:

```text
struct-like whole-payload match binding
    -> UnsupportedFeatureError in the new Semantic IR path
```

The P11 union type and field-projection primitives remain valid and will be used
once struct value identity is canonical.

---

# 13. Borrowed payload boundary

Current Sema already supports:

```sec
Variant(ref value)
Variant(ref mut value)
```

for reusable union/Result subjects and performs borrow-place analysis.

Package 12 does not erase that source feature.

The new Semantic IR path rejects it until reference/borrow representation and
match-scoped reference lifetime semantics exist canonically in Semantic IR.

Do not lower borrowed payload binding as a by-value copy.

---

# 14. Move-only payload boundary

Current Sema may classify a by-value union payload binding as an ownership
transfer.

Package 12 supports only:

```text
copy-trivial
```

by-value payload action.

Any resolved arm requiring move/semantic-copy/conditional-copy is explicit
unsupported in P12.

Do not silently move or clone the payload.

---

# 15. Match subject evaluation

The subject expression is evaluated exactly once before any pattern test.

Semantic IR builder:

```text
subjectValue = build(subject expression)
```

Every pattern test and payload projection uses that same semantic subject value.

Do not rebuild:

```text
call
property access
indexing
conversion
```

for later arms.

---

# 16. Sema resolved MatchPlan

Add a stable read-only plan keyed by `*ast.MatchExpression`.

Recommended subject kinds:

```go
type ResolvedMatchSubjectKind string

const (
    MatchSubjectEnum   ResolvedMatchSubjectKind = "enum"
    MatchSubjectUnion  ResolvedMatchSubjectKind = "union"
    MatchSubjectResult ResolvedMatchSubjectKind = "result"
    MatchSubjectOption ResolvedMatchSubjectKind = "option"
)
```

---

# 17. Resolved pattern kinds

Recommended:

```go
type ResolvedMatchPatternKind string

const (
    MatchPatternEnumValue     ResolvedMatchPatternKind = "enum-value"
    MatchPatternUnionVariant  ResolvedMatchPatternKind = "union-variant"
    MatchPatternResultOk      ResolvedMatchPatternKind = "result-ok"
    MatchPatternResultErr     ResolvedMatchPatternKind = "result-err"
    MatchPatternOptionSome    ResolvedMatchPatternKind = "option-some"
    MatchPatternOptionNone    ResolvedMatchPatternKind = "option-none"
    MatchPatternCatchAll      ResolvedMatchPatternKind = "catch-all"
)
```

Do not expose generic token spellings as pattern semantics.

---

# 18. Binding action

Recommended:

```go
type ResolvedMatchBindingAction string

const (
    MatchBindingNone          ResolvedMatchBindingAction = "none"
    MatchBindingDiscard       ResolvedMatchBindingAction = "discard"
    MatchBindingCopyTrivial   ResolvedMatchBindingAction = "copy-trivial"
    MatchBindingBorrowShared  ResolvedMatchBindingAction = "borrow-shared"
    MatchBindingBorrowMutable ResolvedMatchBindingAction = "borrow-mutable"
    MatchBindingMove          ResolvedMatchBindingAction = "move"
    MatchBindingCopySemantic  ResolvedMatchBindingAction = "copy-semantic"
)
```

Sema records the real resolved action.

P12 builder accepts only:

```text
none
discard
copy-trivial
```

for payload bindings.

---

# 19. Arm flow classification

Recommended:

```go
type ResolvedMatchArmFlow string

const (
    MatchArmProducesValue ResolvedMatchArmFlow = "produces-value"
    MatchArmContinues      ResolvedMatchArmFlow = "continues"
    MatchArmReturns        ResolvedMatchArmFlow = "returns"
    MatchArmTerminates     ResolvedMatchArmFlow = "terminates"
    MatchArmLoopControl    ResolvedMatchArmFlow = "loop-control"
)
```

P12 end-to-end supports:

```text
produces-value
continues
returns
terminates when the contained terminator already has Semantic IR support
```

Loop control waits for loop IR.

---

# 20. ResolvedMatchArm

Recommended:

```go
type ResolvedMatchArm struct {
    SourceIndex             int
    PatternKind             ResolvedMatchPatternKind

    EnumNumericValue        *big.Int
    EnumCaseName            string

    UnionVariantIndex       uint32
    UnionVariantName        string

    BindingName             string
    BindingType             Type
    BindingAction           ResolvedMatchBindingAction

    Guarded                 bool
    Flow                    ResolvedMatchArmFlow
    ResultType              Type

    ResidualAlwaysMatches   bool
}
```

`EnumNumericValue` is authoritative for enum matching.

`EnumCaseName` is provenance/diagnostics.

---

# 21. ResolvedMatchPlan

Recommended:

```go
type ResolvedMatchPlan struct {
    SubjectKind  ResolvedMatchSubjectKind
    SubjectType  Type
    ValueContext bool
    ResultType   Type
    Exhaustive   bool
    Arms         []ResolvedMatchArm
}
```

Optional implementation metadata may include resolved coverage keys.

The plan must not store MLIR types.

---

# 22. Read-only MatchPlan query

Recommended:

```go
func (a *Analyzer) ResolvedMatchPlanOf(
    expr *ast.MatchExpression,
) (ResolvedMatchPlan, bool)
```

The query:

```text
reads facts recorded during successful analysis
does not infer the subject again
does not analyze patterns
does not recompute exhaustiveness
does not mutate Analyzer
```

---

# 23. ResidualAlwaysMatches

Sema records:

```text
ResidualAlwaysMatches
```

when reaching that unguarded arm implies its pattern must match all remaining
possible subject values.

Examples:

```text
final catch-all
final uncovered Result variant
final uncovered Option variant
final uncovered union variant
final enum numeric class only when complete residual-domain proof exists
```

A guarded arm is never residual-always-match solely because its pattern covers
the residual domain; its guard may be false.

The builder does not recompute this proof.

---

# 24. Enum coverage key

For enum arms, duplicate/unreachable/exhaustiveness coverage uses:

```text
canonical arbitrary-precision numeric enum value
```

not case name.

A guarded arm does not mark its value class fully covered.

An unguarded arm does.

Aliases therefore behave correctly.

---

# 25. Semantic IR match provenance

Add function-local:

```go
type MatchID uint32
```

with zero invalid.

Recommended non-executable metadata:

```go
type MatchRecord struct {
    ID           MatchID
    Subject      ValueID
    SubjectType  TypeID
    ResultType   TypeID
    ValueContext bool
    Exhaustive   bool
    Arms         []MatchArmRecord
    MergeBlock   BlockID
    Location     Location
}
```

This record exists for verification/debugging.

Executable semantics remain ordinary CFG.

---

# 26. MatchArmRecord

Recommended:

```go
type MatchArmRecord struct {
    SourceIndex  int
    PatternKind  MatchPatternKind
    PatternBlock BlockID
    GuardBlock   BlockID
    BodyBlock    BlockID
    VariantIndex uint32
    EnumValue    *big.Int
    Guarded      bool
    Flow         MatchArmFlow
    Location     Location
}
```

No AST node is retained.

---

# 27. No Semantic IR MatchOp

Do not add a monolithic:

```text
MatchOp with regions
```

in P12.

The canonical Semantic IR executable form is explicit:

```text
comparisons/variant tests
conditional branches
payload projections
arm bodies
merge block
return/other terminators
```

This exposes control flow to later analyses.

---

# 28. General synthesized unreachable

Add Semantic IR terminator:

```text
UnreachableOp
```

Meaning:

```text
compiler-proven impossible control path
```

Required:

```text
no successors
no results
synthesized = true
reason
location
```

P12 reason:

```text
exhaustive-match-fallthrough
```

It is not:

```text
source panic
Result Err
language undefined behavior
user-visible trap choice
```

If reached due to incorrect compiler reasoning, that is a compiler bug.

---

# 29. Why UnreachableOp is required

P11 guarded union projection requires an actual matching-variant path.

For an exhaustive match chain, the implementation should still test the final
variant and preserve projection proof.

The false edge of the final exhaustive test has no valid source value.

It therefore terminates with synthesized `UnreachableOp`.

Do not remove the final union guard merely to avoid representing the residual
path.

---

# 30. Canonical arm test chain

Conceptual:

```text
subject = evaluate once

arm0.pattern:
    condition0 = test arm0 pattern
    cond_br condition0, arm0.matched, arm1.pattern

arm0.matched:
    establish payload binding if any
    if guard:
        evaluate guard
        cond_br guard, arm0.body, arm1.pattern
    else:
        branch arm0.body

arm0.body:
    execute body
    branch merge / return / terminate
```

Repeat in source order.

After one body is selected, no later arm pattern/guard executes.

---

# 31. Guard evaluation

A guard:

```sec
Pattern(binding) where condition => body
```

is evaluated only after:

```text
pattern matched
payload binding became available
```

The binding is visible inside the guard.

Guard false:

```text
binding scope ends
control continues to the next arm's pattern
```

P12 copy-trivial bindings require no destruction at that point.

Borrowed/moved bindings are deferred.

---

# 32. Catch-all

Catch-all:

```sec
_
```

has no pattern test.

When reached:

```text
branch directly to catch-all body
```

A guarded catch-all:

```sec
_ where condition
```

evaluates the guard and continues to the next arm on false.

It is not exhaustive by itself.

An unguarded catch-all is final live coverage; later source arms are Sema errors.

---

# 33. Enum pattern lowering

For:

```sec
Direction.North
```

P12 uses:

```text
sec.enum.constant
sec.enum.cmp eq
```

The comparison is numeric enum equality.

Case name/ordinal remains provenance.

Alias patterns with the same numeric value therefore naturally test the same
runtime condition.

---

# 34. Union pattern lowering

Payload-less variant:

```text
sec.union.is_variant
```

Single payload variant:

```text
sec.union.is_variant
matching path:
    optionally sec.union.unwrap_payload
```

If payload pattern is `_`:

```text
do not unwrap
```

If binding action is `copy-trivial`:

```text
unwrap and bind result
```

---

# 35. Result pattern lowering

For one Result SSA value, use P10 operations.

`Err` pattern condition:

```text
sec.result.is_err
```

`Ok` pattern:

```text
same is_err condition with branch destinations reversed
```

Matched path:

```text
Ok(binding) -> sec.result.unwrap_ok
Err(binding) -> sec.result.unwrap_err
```

`Ok(_)` does not unwrap solely for a discarded binding.

`Err(_)` remains invalid according to the current Result match rules.

---

# 36. Result repeated guarded arms

Example:

```sec
match result {
    Ok(value) where value > 10 => Large()
    Ok(value) => Normal()
    Err(error) => Failure()
}
```

The second Ok arm remains reachable when the first guard is false.

The builder may reuse a dominating pure `is_err` SSA value if doing so does not
change source evaluation or diagnostics.

It must not reuse payload bindings across arm scopes.

---

# 37. Option pattern lowering

Concrete `Option[T]` uses the canonical P11 union representation:

```text
Some(T)
None
```

Lower:

```text
Some -> sec.union.is_variant + optional unwrap
None -> sec.union.is_variant
```

No special `sec.option.match` operation.

---

# 38. Union/Option payload action

By-value payload binding in P12 requires:

```text
resolved BindingAction == copy-trivial
```

P11 projection action remains:

```text
copy-trivial
```

No hidden move.

---

# 39. Match expression merge

For value-context match, create a merge block:

```text
^merge(%value: T)
```

Every continuing value-producing arm:

```text
cf.br ^merge(%armValue : T)
```

Returning/terminating arms:

```text
no merge edge
```

The Semantic IR/MLIR match expression value is the merge block argument.

---

# 40. Match expression result type

Use Sema's resolved `ResultType`.

The builder does not infer a common branch type.

Exact named/unit distinctions remain as resolved by Sema.

If the type is not yet representable in Semantic IR:

```text
UnsupportedFeatureError
```

Do not coerce it to a nearby builtin type.

---

# 41. Match statement continuation

For statement context, create a continuation block when at least one arm
continues.

Continuing arms branch to it.

Returning/terminating arms do not.

If every exhaustive arm terminates:

```text
no continuation block is required
```

This preserves function return analysis.

---

# 42. Statement expression-arm value boundary

The source match rule permits expression arms in statement context and says
their result is ignored.

P12 new Semantic IR supports this only when the resolved arm expression result
is:

```text
void
or
a non-must-use, trivially destructible immediate value whose unused SSA result
has no ownership/destruction obligation
```

If ignoring the value requires semantic discard/destruction:

```text
UnsupportedFeatureError
```

until canonical `DiscardValue` Semantic IR is implemented.

Do not use unused SSA as an implicit discard for owned/must-use values.

---

# 43. Return arm

`return` inside a match arm lowers using the existing Semantic IR return
operation.

No branch to match merge/continuation.

For Result returns, existing P9/P10 Result construction remains applicable.

---

# 44. Terminating arm

A Sema-resolved terminating arm lowers only if its terminator is already
representable in Semantic IR.

Examples later may include:

```text
panic
unreachable
```

but P12 does not invent terminators for unsupported source statements.

---

# 45. Loop control boundary

Source `continue` inside match remains valid when enclosed by a loop.

P12 new IR reports explicit unsupported if the target loop control-flow
operation is not yet represented.

Do not mis-lower `continue` as match continuation.

---

# 46. Exhaustive fallthrough

The match plan is exhaustive.

The final pattern-test false path, when still materialized, ends in:

```text
UnreachableOp(reason = exhaustive-match-fallthrough)
```

This path is synthesized and has no source observability.

A final unguarded catch-all needs no unreachable tail.

---

# 47. Semantic IR match verifier

Verify MatchRecord against CFG.

Required:

```text
subject ValueID exists
subject is evaluated before the first pattern test
all pattern tests use the same subject
arm SourceIndex values are strictly source ordered
pattern false edge leads to next source arm test
pattern true edge leads to matched/guard/body path
guard exists only when plan says Guarded
guard result type is bool
guard false edge leads to next source arm test
body selection has no edge to later arm tests
copy-trivial binding type matches projection
expression merge values equal resolved ResultType
return/terminate arms have no merge edge
exhaustive chain cannot fall into ordinary continuation
synthesized residual tail is UnreachableOp when needed
```

---

# 48. Enum-specific MatchPlan verifier

Verify:

```text
EnumNumericValue is representable by enum underlying domain
unguarded duplicate numeric coverage is absent
guarded numeric arm may precede same numeric class
Exhaustive true follows corrected numeric-domain rules
catch-all covers all residual numeric values
case name is provenance only
```

---

# 49. Union-specific MatchPlan verifier

Verify:

```text
variant index exists
variant name/index agree
payload binding only for payload-carrying variant
empty variant has no binding
single payload binding type matches
copy-trivial gate obeyed
struct-like binding rejected by P12 builder
```

---

# 50. Result-specific MatchPlan verifier

Verify:

```text
Ok/Err subject is Result
Ok binding type equals T
Err binding type equals E
Ok discard has no binding
Err discard _ is absent
catch-all error-hiding rule already satisfied by Sema
```

---

# 51. Sec MLIR schema version 8

Compiler-generated high-level Sec MLIR uses:

```mlir
sec.dialect_version = 8 : i32
```

Schema 8 adds one general terminator:

```text
sec.unreachable
```

and reserves transient compiler-generated match provenance attributes:

```text
sec.match_id
sec.match_arm_index
sec.match_stage
sec.match_pattern_kind
```

No `sec.match` operation is introduced.

---

# 52. `sec.unreachable`

No operands.

No results.

No successors.

Terminator.

Required attributes:

```text
sec.synthesized = true
reason: StringAttr
```

Package 12 canonical reason:

```text
exhaustive-match-fallthrough
```

Optional:

```text
sec.match_id
```

for verifier provenance.

---

# 53. `sec.unreachable` semantics

The operation means:

```text
this control path is proven impossible by higher-level Sec semantics
```

It is not a source panic.

It is not an implicit Result error.

It is not a promise that invalid source behavior is allowed.

The compiler must verify the proof-producing structure before lowering it to a
backend-level unreachable primitive.

---

# 54. Match provenance attributes

Allowed values:

```text
sec.match_id: positive i32
sec.match_arm_index: non-negative i32

sec.match_stage:
    pattern
    guard
    body-exit
    merge
    residual

sec.match_pattern_kind:
    enum-value
    union-variant
    result-ok
    result-err
    option-some
    option-none
    catch-all
```

These attributes are verifier/debug provenance.

They do not determine source semantics.

---

# 55. Standard MLIR CFG

Use:

```text
cf.cond_br
cf.br
MLIR block arguments
func.return
```

No fallthrough.

No `switch` conversion is required.

Do not use `cf.switch` merely because a subject is enum/union; source-order
guards and payload scopes make explicit CFG the canonical P12 form.

A later optimization may create a switch after proving equivalence.

---

# 56. MLIR enum pattern

Canonical conceptual fragment:

```mlir
%case = "sec.enum.constant"() ... : () -> !sec.enum<...>
%matches = "sec.enum.cmp"(%subject, %case) {predicate = "eq"}
    : (!sec.enum<...>, !sec.enum<...>) -> i1
cf.cond_br %matches, ^matched, ^next
```

The case constant carries declared case provenance.

Comparison semantics are numeric enum equality.

---

# 57. MLIR union pattern

```mlir
%matches = "sec.union.is_variant"(%subject)
    {variant = 1 : i32}
    : (!sec.union<...>) -> i1

cf.cond_br %matches, ^matched, ^next

^matched:
    %payload = "sec.union.unwrap_payload"(%subject)
        {variant = 1 : i32, action = "copy-trivial"}
        : (!sec.union<...>) -> T
```

The existing union guard verifier must accept this canonical pattern.

---

# 58. MLIR Result pattern

```text
sec.result.is_err
cf.cond_br
sec.result.unwrap_ok / unwrap_err
```

Use the same subject SSA.

The existing Result guard verifier must be generalized to tolerate source-order
match chains with repeated tests, not only one fixed two-block try shape.

---

# 59. Result guard verifier compatibility

P10's Result guard verifier was initially designed around a canonical direct
Err/Ok branch.

Package 12 extends it to recognize:

```text
match pattern test sites
guarded repeated Ok/Err pattern tests
payload projection on the matching branch
```

It still rejects:

```text
Ok unwrap on a proven Err path
Err unwrap on a proven Ok path
projection from a different Result SSA value
```

---

# 60. Union guard verifier compatibility

P11 union guard verification must recognize matching blocks generated by P12
and nested guard blocks that remain dominated by the true variant-test edge.

A guard false edge may leave the variant scope and continue to later patterns.

The payload projection itself stays on a proven matching path.

---

# 61. Match CFG verifier

Register:

```bash
--sec-verify-match-cfg
```

Function-level verification pass.

It uses:

```text
Sec pattern operations
cf edges
match provenance attributes
dominance
schema-v8 sec.unreachable
```

It does not redo source exhaustiveness.

---

# 62. Match CFG verifier requirements

Required checks:

```text
match IDs are function-local and positive
arm indexes are source ordered
pattern/guard stages are consistent
pattern false edge advances to later arm
guard false edge advances to later arm
body exit never jumps to a later pattern
catch-all has no pattern comparison
unguarded catch-all has no later live arm
expression merge receives exact type/arity
statement continuation receives no hidden value
residual impossible edge ends in synthesized sec.unreachable
sec.unreachable reason is exhaustive-match-fallthrough
```

---

# 63. No match verifier source reconstruction

The verifier may use emitted resolved provenance.

It must not:

```text
parse enum case spelling
infer union variant from block name
guess source arm order from block numbering alone
reanalyze AST
```

---

# 64. Sema enum coverage changes

Change the current enum match coverage structures from:

```text
seen variant names
```

to:

```text
seen unguarded numeric coverage keys
```

for exhaustiveness/duplicate reachability.

Keep declaration names for diagnostics.

---

# 65. Sema union/Result coverage

Existing variant coverage rules remain conceptually valid:

```text
union coverage by variant identity
Result coverage by Ok/Err
Option coverage by Some/None
```

Guarded arms do not count as full coverage.

---

# 66. Source order and guards

The resolved plan preserves AST arm order exactly.

No builder sorting by:

```text
variant index
enum numeric value
guard presence
payload size
```

is permitted.

Optimization belongs later.

---

# 67. Binding scope

Pattern binding scope:

```text
guard
selected arm body
```

only.

It is not visible:

```text
in later arm tests
in later guards
after match
```

The builder uses function-local binding environment push/pop per matched arm.

---

# 68. Guard side effects

Guard expression evaluation may have ordinary language effects.

Therefore:

```text
do not speculate a guard
do not evaluate a guard when its pattern is false
do not evaluate later guards after an arm is selected
```

P12 correctness must hold without optimization assumptions.

---

# 69. Match subject effects

Because subject is evaluated once:

```sec
match ReadResult() {
    ...
}
```

calls `ReadResult()` once.

Pattern tests are pure semantic tests on the resulting SSA value.

---

# 70. Match expression evaluation order

For a selected arm:

```text
subject
earlier pattern tests
earlier matching guards that evaluate false
selected pattern test
selected payload binding/projection
selected guard
selected body
```

Later arm patterns/guards/bodies are not evaluated.

---

# 71. Result/Option error semantics

`match` does not implicitly propagate errors.

Result Err must be handled according to existing Result match rules.

Catch-all must not silently hide Err where forbidden by Sema.

P12 consumes Sema's resolved valid plan.

It does not weaken error-sensitive match rules.

---

# 72. Match and try reuse

P10 local try handlers and P12 match both use explicit CFG.

They may share internal helper code for:

```text
source-order test chains
guard branches
value merge construction
return/terminate arm exits
```

Do not merge their source semantic plan types.

`try` and `match` remain distinct language constructs.

---

# 73. No physical enum lowering

P12 enum matching remains:

```text
sec.enum.constant
sec.enum.cmp
```

It does not lower the enum to an integer solely to build match CFG.

Enum representation erasure remains later.

---

# 74. No physical union lowering

P12 union matching remains:

```text
sec.union.is_variant
sec.union.unwrap_payload
```

No tag integer, payload buffer or LLVM representation is introduced.

---

# 75. No hidden runtime dispatch

Match CFG is static.

No:

```text
runtime pattern table
reflection
variant-name lookup
exception dispatcher
heap allocation
```

is required.

---

# 76. Required Sema tests - enum correction

Add/update:

```text
alias numeric duplicate unguarded arm rejected
guarded alias then value-equivalent unguarded arm accepted
ordinary enum declared cases without catch-all non-exhaustive when unnamed
numeric values remain possible
bit[1] with both numeric values covered may be exhaustive
bit[2] all four numeric values covered may be exhaustive
bit[2] missing one numeric value non-exhaustive
catch-all makes enum exhaustive
guarded enum arm does not count as full numeric coverage
int128/uint256 underlying coverage uses arbitrary precision
```

Update older tests that assumed declared case-name coverage was automatically
exhaustive.

---

# 77. Required Sema tests - match plan

Verify read-only plan records:

```text
subject kind/type
value context
resolved result type
source arm indexes
pattern kinds
enum numeric values
union variant indexes
binding names/types/actions
guarded flag
flow classification
exhaustive flag
ResidualAlwaysMatches
```

Calling the query repeatedly must not mutate Sema.

---

# 78. Required Semantic IR tests - enum

```text
subject evaluated once
enum source-order tests
enum guard false reaches next arm
enum catch-all
enum match expression merge
enum return arm
enum alias numeric plan
synthesized unreachable residual when required
```

---

# 79. Required Semantic IR tests - union

```text
payload-less variants
single trivial payload binding
single payload discard _
variant guard
guarded repeated same variant followed by unguarded variant
catch-all
match expression merge
int128 payload
uint256 payload
unguarded projection rejected by verifier
ref/ref mut payload explicit unsupported
move-only payload explicit unsupported
struct-like whole-payload explicit unsupported
```

---

# 80. Required Semantic IR tests - Result/Option

```text
Result Ok binding
Result Ok discard
Result Err named binding
Result guarded Ok followed by unguarded Ok
Result exhaustive Ok/Err
Result error-sensitive catch-all rules remain frontend-owned

Option Some binding
Option Some discard
Option None
Option guarded Some followed by Some
Option exhaustive Some/None
```

---

# 81. Required MLIR dialect tests

Schema v8:

```text
sec.unreachable round-trip
sec.unreachable requires synthesized=true
sec.unreachable requires non-empty reason
sec.unreachable is terminator
schema-v7 regression remains valid
```

No new enum/union operation syntax is introduced by P12.

---

# 82. Required Match CFG verifier tests

```text
canonical enum chain accepted
canonical union chain accepted
canonical Result chain accepted
canonical Option chain accepted
subject mismatch rejected
arm index disorder rejected
pattern false edge skipping/reordering rejected
guard false edge wrong target rejected
body exit to later pattern rejected
catch-all with later arm rejected
wrong merge arity/type rejected
ordinary continuation from exhaustive residual rejected
missing sec.unreachable residual rejected when required
wrong unreachable reason rejected
```

---

# 83. Result guard compatibility tests

```text
repeated guarded Ok patterns accepted
repeated guarded Err patterns accepted
Ok projection on matching path accepted
Err projection on matching path accepted
opposite projection rejected
different Result SSA projection rejected
guard block remains in matching dominance region
```

---

# 84. Union guard compatibility tests

```text
variant projection before guard rejected
variant projection in matching block accepted
variant projection in guard/body dominated by matching edge accepted
guard false escape to next pattern accepted
projection after guard-false join rejected
```

---

# 85. End-to-end enum examples

Required:

```text
enum with catch-all statement match
enum expression match
enum guard
enum alias diagnostic
bit[1] full-domain exhaustive match
bit[2] full-domain exhaustive match
int128-underlying enum match
uint256-underlying enum match
nested enum match
```

---

# 86. End-to-end union examples

Required:

```text
payload-less State match
Option[int32] Some/None
Option[int128] Some/None
single uint256 payload union
guarded single-payload union
payload discard
catch-all
match expression returning int32
match expression returning int128
```

---

# 87. End-to-end Result examples

Required:

```text
Result[int32, ArithmeticError] match
Result[int128, IOError-enum] match
Result[uint256, IOError-enum] match
guarded Ok
guarded Err
expression match with returning Err arm
```

Use P11 canonical enum error representation.

---

# 88. Unsupported end-to-end tests

Must fail with explicit new-pipeline unsupported errors:

```text
struct-like union payload binding
ref payload binding
ref mut payload binding
move-only payload binding
semantic-copy payload binding
nested pattern
literal/range pattern
loop continue from match before loop IR
statement arm whose ignored result requires semantic discard/destruction
```

Do not emit placeholder or unsound IR.

---

# 89. Compiler commands

`sec emit-ir`:

```text
prints explicit match CFG and synthesized unreachable
```

`sec emit-sec-mlir`:

```text
emits schema-v8 explicit cf match CFG
runs applicable verifier pipeline
```

Legacy:

```text
emit-mlir
emit-llvm
build
```

remain unchanged.

---

# 90. Verification pipeline

Compiler-generated schema-v8 high-level Sec MLIR runs as applicable:

```text
normal MLIR verifier
sec-verify-checked-integer-guards
sec-verify-result-guards
sec-verify-try-handlers
sec-verify-union-guards
sec-verify-match-cfg
```

Do not use unregistered-dialect mode.

---

# 91. Package 8 compatibility

Integer operations inside:

```text
enum guard
match guard
match arm body
```

may later lower through the existing checked integer Arith pipeline.

P12 match CFG remains ordinary `cf`.

No P8 pass may reorder source match arms as part of integer lowering.

---

# 92. P10 compatibility

Result projections now appear both in:

```text
try CFG
match CFG
```

The Result guard verifier must distinguish supported canonical forms by
provenance/structure without treating one as invalid merely because it is not
the original P10 two-way shape.

---

# 93. P11 compatibility

P12 consumes P11:

```text
!sec.enum
sec.enum.constant
sec.enum.cmp

!sec.union
sec.union.is_variant
sec.union.unwrap_payload
```

It does not require physical layout.

P11 `sec.union.unwrap_field` is not sufficient to fake a whole struct-like
payload binding; that source form remains deferred.

---

# 94. Architecture rules

Non-negotiable:

```text
Match subject is evaluated exactly once.

Arms are considered in source order.

First successful pattern+guard wins.

A guard is evaluated only after its pattern matches.

Guard false continues with the next arm.

No later arm executes after selection.

Sema owns pattern meaning, reachability, exhaustiveness and result typing.

Builder consumes ResolvedMatchPlan.

Enum match is numeric-value semantic, not case-provenance semantic.

Enum aliases with equal numeric value match the same runtime domain.

Enum exhaustiveness includes unnamed values allowed by explicit conversion.

Union matching uses semantic variant identity, not physical tag integers.

Payload projection remains guarded.

By-value payload support is copy-trivial only in P12.

Borrow/move semantics are never faked.

Result/Option matching reuses canonical high-level representations.

Expression match uses SSA merge.

Exhaustive impossible residual control uses synthesized sec.unreachable.

sec.unreachable is not source panic.

No physical enum/union layout is selected.

No runtime dispatcher is introduced.

No LLVM dialect is generated.
```

---

# 95. Acceptance criteria

Package 12 is complete only when:

```text
[ ] repository baseline 152c772 or newer sync documented
[ ] previous regressions remain green
[ ] wide builtin invariant remains
[ ] enum-domain normative amendment applied
[ ] Semantic IR match amendment applied
[ ] dialect schema v8 installed
[ ] lowering spec v8 installed
[ ] enum alias coverage corrected to numeric value classes
[ ] unnamed enum values reflected in exhaustiveness
[ ] finite full-domain enum proof implemented
[ ] ResolvedMatchPlan API implemented
[ ] MatchPlan query is read-only
[ ] subject evaluated exactly once
[ ] source arm order preserved
[ ] guard order preserved
[ ] guard evaluated only after match
[ ] first-match semantics preserved
[ ] enum match lowering implemented
[ ] union empty variant lowering implemented
[ ] union copy-trivial single payload lowering implemented
[ ] union payload discard implemented
[ ] Result match lowering implemented
[ ] Option match lowering implemented
[ ] catch-all implemented
[ ] expression SSA merge implemented
[ ] statement continuation implemented
[ ] return/terminate arm separation implemented
[ ] Semantic IR UnreachableOp implemented
[ ] sec.unreachable implemented
[ ] exhaustive residual path uses synthesized unreachable
[ ] match provenance metadata emitted
[ ] --sec-verify-match-cfg registered
[ ] Result guard verifier accepts match form
[ ] union guard verifier accepts match form
[ ] ref/ref mut payload remains explicit unsupported
[ ] move-only payload remains explicit unsupported
[ ] struct-like whole-payload binding remains explicit unsupported
[ ] no general literal/range matching added
[ ] no physical enum/union lowering added
[ ] check-sec-mlir passes
[ ] go test ./... passes
[ ] legacy paths remain operational
```

---

# 96. Required implementation report

Codex must report:

```text
1. repository HEAD implemented against
2. previous package status
3. enum-domain rule correction applied
4. files added
5. files modified
6. Sema ResolvedMatchPlan API
7. enum numeric coverage implementation
8. enum full-domain exhaustiveness proof
9. Semantic IR MatchRecord representation
10. subject-once builder algorithm
11. source-order pattern-chain algorithm
12. guard CFG algorithm
13. enum pattern lowering
14. union pattern/payload lowering
15. Result pattern lowering
16. Option pattern lowering
17. expression merge algorithm
18. statement continuation algorithm
19. UnreachableOp implementation
20. schema-v8 sec.unreachable
21. match provenance attributes
22. match CFG verifier
23. Result verifier compatibility changes
24. union verifier compatibility changes
25. alias tests
26. wide enum tests
27. wide union/Result/Option tests
28. unsupported ownership/struct cases
29. CMake commands
30. exact LLVM/MLIR version
31. check-sec-mlir result
32. go test ./... result
33. end-to-end source -> schema-v8 results
34. deviations
35. recommendations for Package 13
```

---

# 97. Package 13 boundary

Recommended Package 13:

```text
Struct Semantic Value Representation
```

Reason:

P12 exposes the next concrete blocker:

```text
struct-like union payload binding
ordinary struct values
field access/projection
field-wise construction
```

A canonical struct layer also unlocks:

```text
struct-like union payload match binding
larger Result/error payloads
aggregate function arguments/results
future array/struct ownership work
```

Recommended scope:

```text
Semantic IR struct type definition
field identity/order
struct construction
omitted/default fields using already-resolved defaults
field projection
mutable field storage boundary
Sec MLIR !sec.struct
Sec MLIR struct construct/project operations
nested struct identity
trivial-copy struct subset
struct-like union payload materialization using canonical struct type
no physical LLVM struct layout yet
```

Package 13 should still defer:

```text
non-trivial ownership/copy/destruction
general aggregate ABI lowering
arrays/slices
LLVM
```

After P13, P12 can remove the struct-like union payload match limitation without
a match-specific special representation.
