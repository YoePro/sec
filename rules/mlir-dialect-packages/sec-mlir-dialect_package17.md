# Sec MLIR Program - Implementation Package 17

## Package status

Implementation package for the Sec compiler.

Package ID: `SEC-MLIR-P17`  
Package title: `Ownership Transfer and Destruction Semantic Core`  
Repository: `https://github.com/YoePro/sec`  
Repository branch: `main`  
Repository sync commit used for this package: `152c772`  
Local predecessors: `SEC-MLIR-P13`, `SEC-MLIR-P14`, `SEC-MLIR-P15`, `SEC-MLIR-P16`  
Repository sync date: `2026-08-10`  
Semantic IR version before package: `1`  
Semantic IR version after package: `1`  
Sec MLIR dialect schema before package: `12`  
Sec MLIR dialect schema after package: `13`  
Sec MLIR lowering specification before package: `12`  
Sec MLIR lowering specification after package: `13`

Package 17 establishes the canonical compiler representation for ownership
transfer and deterministic destruction.

It replaces the temporary `copy-trivial only` implementation boundary used by
Packages 11-16 with explicit semantic operations for:

```text
construction ownership
trivial copy
infallible semantic copy
move
move from storage/place
initialization
reinitialization after move
replacement
partial struct/payload move
destruction
discard
cleanup registration
defer registration
scope cleanup
function-exit cleanup
```

The package keeps source syntax simple.

Ordinary source code still does not require `move` or `copy` keywords.

Sema resolves the transfer action and Semantic IR records it explicitly.

---

# 1. Normative authority

Implementation follows:

```text
rules/ownership.md
rules/copy_move.md
rules/destruction.txt
rules/defer.txt
rules/discard.md
rules/borrowing.txt
rules/errorhandling.txt
rules/semantic_ir.txt
    ↓
local P13-P16 normative amendments
    ↓
rules/sec_mlir.md
rules/sec_mlir_dialect.md
rules/sec_mlir_lowering.md
    ↓
implementation package
    ↓
implementation
```

Before implementation:

1. apply `sec_ownership_destruction_sync_package17.md`;
2. apply `sec_semantic_ir_ownership_destruction_package17.md` to
   `rules/semantic_ir.txt`;
3. update `rules/sec_mlir_dialect.md` with
   `sec_mlir_dialect_package17.md`;
4. update `rules/sec_mlir_lowering.md` with
   `sec_mlir_lowering_package17.md`.

No new source-level `move`, `copy`, `free`, `replace`, `take`, or destructor
syntax is introduced by P17.

---

# 2. Repository and local predecessor rule

GitHub `main` is still:

```text
152c772
```

P17 additionally assumes the local package series:

```text
P13:
    canonical structs

P14:
    canonical fixed arrays

P15:
    places and direct safe references

P16:
    slices
```

If those packages are merged before implementation under a newer HEAD, Codex
must report the new HEAD and verify semantic equivalence.

---

# 3. Wide builtin invariant

These remain active Sec builtin types:

```text
int128
int256
uint128
uint256
decimal128
```

P17 does not change their status.

They remain trivially copyable/destructible unless another type wrapper adds
ownership behavior.

---

# 4. Core ownership rule

For every source use that may affect ownership, the compiler must already know:

```text
source value/place
source type
destination context
copy classification
destruction classification
borrow state
whether the use consumes ownership
whether the source remains initialized
whether the destination becomes initialized
```

No lower stage may infer ownership from:

```text
LLVM load/store
MLIR SSA multiplicity
memcpy
physical register copies
pointer equality
```

---

# 5. Copy/move classifications

P17 uses the canonical type classifications:

```text
CopyTrivial
CopySemanticInfallible
MoveOnly
CopyConditional
NoCopy
```

The exact internal enum names may follow the existing compiler.

`CopyConditional` must be resolved to a concrete classification before runtime
Semantic IR for a concrete monomorphized value.

---

# 6. Infallible semantic copy only

P17 `SemanticCopyOp` represents only:

```text
infallible
semantically defined
source-preserving
ownership-correct
```

copy behavior.

A fallible clone/duplication remains an explicit ordinary function call such as:

```text
Clone(...) -> Result[T, E]
```

P17 must not hide allocation failure or other errors inside assignment,
parameter passing, or implicit copy.

---

# 7. Move cannot fail

A pure ownership move:

```text
cannot fail
```

Operations that consume a value and also perform a fallible action define their
failure ownership through their normal API types.

P17 never invents ownership rollback after a move.

---

# 8. Source syntax remains implicit

Examples:

```sec
let second := first
Consume(value)
return value
```

are resolved by Sema as:

```text
copy
move
return-transfer
```

according to type/context.

The programmer does not need a routine `move` keyword.

---

# 9. Ownership value identity

Semantic IR uses ordinary SSA value identity plus explicit ownership operations.

An owned SSA value has one current semantic owner on each reachable control-flow
path.

P17 does not require a runtime ownership token.

Recommended compile-time identity:

```go
type OwnershipID uint32
```

may be used in Sema/Semantic IR diagnostics and analysis tables.

In MLIR, SSA value identity plus explicit ownership operations and place/storage
identity are sufficient for verification.

---

# 10. Ownership state

Canonical analysis states:

```go
type OwnershipState string

const (
    OwnershipUninitialized OwnershipState = "uninitialized"
    OwnershipInitialized   OwnershipState = "initialized"
    OwnershipMoved         OwnershipState = "moved"
    OwnershipDestroyed     OwnershipState = "destroyed"
    OwnershipPartial       OwnershipState = "partial"
)
```

A state belongs to a value or storage incarnation, not merely a source name.

---

# 11. Subobject state

For aggregates that support partial ownership tracking, retain per-subobject
state.

P17 required subobject domains:

```text
struct stored fields
active union payload
Result active payload
Option Some payload
```

P17 does not add arbitrary runtime-index array-element partial move.

Fixed-array element partial move remains deferred unless the index is represented
by a later dedicated ownership-safe operation.

---

# 12. Partial struct move

P17 implements the initial direct-field policy.

A stored struct field may be moved out when:

```text
source is an owned supported place
field is directly addressable
aggregate has no custom free operation
no active conflicting borrow exists
remaining fields can be tracked
no hidden ownership invariant forbids it
storage is not volatile/register/opaque foreign/RawPtr-derived
```

Afterward:

```text
moved field unavailable
remaining fields remain owned
whole struct unavailable as a complete value
destruction skips moved field
```

---

# 13. Field reinitialization

A mutable partially moved struct may become complete again.

Reinitializing a moved field:

```text
does not destroy the moved old field
consumes/constructs the replacement into that field
updates field initialization state
may restore whole-value availability
```

P17 implements this for the supported direct-field place subset.

---

# 14. Union/Result/Option payload move

A payload may be moved from:

```text
union
Result
Option
```

only when the active variant is proven on the path.

After payload move:

```text
payload is unavailable
container cannot be used as a complete value unless source rules restore it
destruction must not destroy the moved payload
```

This enables P12 move-only by-value match payload binding for the supported
owned-subject cases.

---

# 15. Array partial-move boundary

P17 does not introduce arbitrary partial move from fixed arrays.

Reason:

```text
runtime index identity
per-element initialization masks
large-array state
```

require a dedicated design.

Whole fixed arrays may move.

Array construction/destruction may be non-trivial.

---

# 16. Ownership transfer plans

Add a read-only Sema transfer plan.

Recommended:

```go
type ResolvedTransferAction string

const (
    TransferConstruct       ResolvedTransferAction = "construct"
    TransferCopyTrivial     ResolvedTransferAction = "copy-trivial"
    TransferCopySemantic    ResolvedTransferAction = "copy-semantic-infallible"
    TransferMove            ResolvedTransferAction = "move"
    TransferBorrowShared    ResolvedTransferAction = "borrow-shared"
    TransferBorrowMutable   ResolvedTransferAction = "borrow-mutable"
    TransferNonConsuming    ResolvedTransferAction = "non-consuming"
)
```

---

# 17. `ResolvedTransferPlan`

Recommended:

```go
type ResolvedTransferPlan struct {
    Action          ResolvedTransferAction
    Type            Type
    SourceValue     ValueIdentity
    SourcePlace     *Place
    DestinationKind TransferDestinationKind
    DestinationID   uint32
    PartialPath     *Place
    OwnershipBefore OwnershipState
    OwnershipAfter  OwnershipState
    Location        SourceLocation
}
```

Exact compiler types may differ.

The Semantic IR builder consumes this plan.

It must not reclassify copy versus move.

---

# 18. Transfer-plan queries

Provide read-only queries keyed by the resolved source/use node.

Possible APIs:

```go
ResolvedTransferPlanOf(node ast.Node)
ResolvedAssignmentOwnershipPlanOf(node *ast.Assignment)
ResolvedCallOwnershipPlanOf(node *ast.CallExpression)
ResolvedReturnOwnershipPlanOf(node *ast.ReturnStatement)
```

A consolidated API is acceptable.

Requirements:

```text
no Analyzer mutation
no repeated ownership classification
no borrow-state mutation
no move-state mutation
```

---

# 19. Destruction classification

Every concrete runtime type has:

```text
TriviallyDestructible
or
resolved non-trivial destruction plan
```

No non-trivial value may reach high-level MLIR without a destruction plan.

---

# 20. `DestructionPlanID`

Add:

```go
type DestructionPlanID uint32
```

Recommended destruction kinds:

```go
type DestructionKind string

const (
    DestroyTrivial       DestructionKind = "trivial"
    DestroyStruct        DestructionKind = "derived-struct"
    DestroyFixedArray    DestructionKind = "derived-fixed-array"
    DestroyUnion         DestructionKind = "active-union"
    DestroyResult        DestructionKind = "active-result"
    DestroyOption        DestructionKind = "active-option"
    DestroyCompilerKnown DestructionKind = "compiler-known"
    DestroyCustomFree    DestructionKind = "custom-free"
)
```

---

# 21. `ResolvedDestructionPlan`

Recommended:

```go
type ResolvedDestructionPlan struct {
    ID                  DestructionPlanID
    Type                Type
    Kind                DestructionKind
    Trivial             bool
    FieldPlans          []DestructionPlanID
    ElementPlan         DestructionPlanID
    VariantPlans        []DestructionPlanID
    CustomFreeSymbol    SymbolID
    DeallocationPlan    AllocationReleasePlan
    Location            SourceLocation
}
```

Do not encode physical layout.

---

# 22. Custom free source boundary

P17 provides IR support for a resolved custom-free destruction implementation.

However current destruction rules still require the initial compiler to reject
user-defined `free` syntax until parser/Sema syntax and restrictions are
explicitly implemented.

P17 therefore:

```text
defines the destruction-plan hook
does not broaden source acceptance of custom free declarations
```

Compiler-known or future resolved types may use the same hook.

---

# 23. Derived struct destruction

Without custom free:

```text
destroy initialized, unmoved stored fields
in reverse declaration order
```

Properties/methods/nested types are not destroyed as fields.

A partially moved struct skips moved fields.

---

# 24. Fixed-array destruction

For:

```text
T[N]
```

destroy initialized non-trivial elements in:

```text
N-1
N-2
...
0
```

order.

Large arrays may later lower to a loop.

P17 does not expand a million-element array into a million MLIR destroy ops.

---

# 25. Union destruction

Destroy only:

```text
the active initialized unmoved payload
```

No inactive variant payload is touched.

---

# 26. Result/Option destruction

Result:

```text
destroy active Ok or Err payload according to its type
```

Option:

```text
Some -> destroy payload
None -> no payload destruction
```

Moved payloads are skipped.

---

# 27. Reference/slice destruction

Direct references and slices:

```text
do not destroy referents
```

Their semantic lifetime end may require:

```text
sec.ref.end_borrow
```

but no referent `DestroyOp`.

---

# 28. Discard becomes real ownership semantics

P17 implements Semantic IR discard.

`discard expression`:

```text
evaluates once
consumes the resulting owned value
runs deterministic destruction when required
marks binding/place unavailable when applicable
```

Implicit discard of ordinary call results uses the same destruction engine.

---

# 29. Explicit versus implicit discard provenance

Preserve:

```text
explicit
implicit-call-result
compiler-temporary
```

discard origin.

Must-use legality remains Sema-owned.

The backend never decides whether discard was legal.

---

# 30. Reference discard

Discard of:

```text
ref T
ref mut T
shared slice
mutable slice
```

ends the reference value/borrow holder according to P15/P16 facts.

It does not destroy backing storage.

---

# 31. Assignment replacement

Normal assignment to initialized owned storage is replacement.

Canonical semantics:

```text
1. evaluate RHS completely
2. resolve/perform copy or move into a temporary ownership result
3. preserve source/alias/borrow validity
4. destroy old destination value
5. initialize destination with new ownership
6. register new destruction responsibility
```

P17 represents the replacement explicitly.

---

# 32. Transactional fallible replacement

For a fallible RHS:

```text
old destination remains unchanged on failure
```

Canonical control flow:

```text
evaluate RHS
failure -> existing error path, old cleanup remains active
success -> replace old destination
```

No early destination destruction.

---

# 33. Reinitialization after whole move

A mutable binding whose value was moved may be initialized again.

This is:

```text
initialization
```

not replacement.

No old destruction occurs.

The new value registers a new cleanup responsibility at the reinitialization
point.

---

# 34. Cleanup ordering decision

P17 selects the destruction rulebook's recommended unified cleanup-registration
model as canonical.

Cleanup registration events include:

```text
successful initialization of an owned non-trivial value
executed defer statement
reinitialization that creates a new owned value
replacement after old ownership has terminated
```

Cleanup executes in reverse registration order, subject to scope/function-exit
eligibility.

This is observable language order.

---

# 35. Cleanup stack is semantic, not mandatory runtime

The conceptual cleanup stack may be:

```text
fully compile-time expanded
represented by high-level Semantic IR
implemented by generated local storage for dynamic defer registration
optimized away
```

No global cleanup runtime is required.

---

# 36. `CleanupActionID`

Add:

```go
type CleanupActionID uint32
type CleanupScopeID uint32
type DeferID uint32
```

Cleanup actions are compiler semantic identities.

They are not source values.

---

# 37. Cleanup action kinds

Required:

```text
destroy-owned
defer
end-borrow where tied to cleanup
compiler-temporary-destroy
```

Future allocator/deallocation actions may be nested in a destruction plan.

---

# 38. Cleanup registration

After successful initialization of a non-trivial owned local:

```text
register destruction cleanup
```

If ownership later moves out:

```text
cancel/consume that cleanup responsibility
```

If the storage is reinitialized:

```text
register a new cleanup action at the new initialization point
```

---

# 39. Scope cleanup

Leaving a lexical scope normally, by break, or by continue executes:

```text
eligible automatic cleanup actions for the scopes being exited
```

Function-scoped defer registrations remain pending.

A defer does not execute merely because its lexical block ends.

---

# 40. Function-exit cleanup

Before normal function return:

```text
1. evaluate return expression
2. establish return ownership transfer
3. execute remaining cleanup entries in reverse registration order
4. return to caller
```

Error propagation follows the same rule after preparing the propagated Err.

The transferred return/error ownership is not destroyed by callee cleanup.

---

# 41. Unified defer/destruction order example

Source:

```sec
let first := First.Create()

defer {
    Log("leaving")
}

let second := Second.Create()
```

Conceptual registration:

```text
destroy first
defer Log
destroy second
```

Function-exit order:

```text
destroy second
defer Log
destroy first
```

---

# 42. Defer captures binding/place identity

A defer does not copy its referenced values at registration.

P17 represents defer captures as:

```text
binding/place identity
```

or equivalent addressable semantic reference to the binding instance.

At execution the defer body reads the then-current value of that binding.

---

# 43. Defer lifetime extension

A binding/place referenced by a registered defer must remain semantically valid
until that defer registration executes.

The compiler may therefore extend physical storage lifetime.

This is not an ownership copy.

---

# 44. Defer in loops

Each execution of a defer statement creates one semantic registration.

A loop-iteration binding that is a distinct semantic binding instance remains
distinct for each registration.

P17 must not collapse registrations merely because source spelling is reused.

Dynamic registration may require high-level cleanup-stack representation until
physical lowering.

---

# 45. Defer body restrictions remain frontend-owned

P17 assumes Sema already enforces:

```text
no return/break/continue/fallthrough from defer
no nested defer
no error propagation from defer
all Result failures handled locally
normal ownership/borrow rules
```

MLIR verification checks structural invariants but does not reparse source
control flow.

---

# 46. Panic boundary

Current Sec rules do not define panic unwinding.

Therefore P17 does not insert ordinary function/scope cleanup merely because a
panic endpoint is reached.

Existing high-level panic endpoints such as:

```text
sec.fail.bounds
sec.fail.reference_generation
sec.fail.arithmetic / later panic endpoint
```

remain non-returning.

No hidden exception unwinding is introduced.

---

# 47. Raw process-exit/unreachable boundary

Do not claim cleanup after:

```text
raw non-returning process-exit syscall
compiler-proven unreachable
abnormal target termination
```

unless the operation explicitly defines graceful cleanup.

---

# 48. Semantic IR ownership operations

P17 adds:

```text
CopyValueOp
SemanticCopyValueOp
MoveValueOp
CopyFromPlaceOp
MoveFromPlaceOp
InitializePlaceOp
ReplacePlaceOp
DiscardValueOp
DestroyValueOp
DestroyPlaceOp
```

Existing borrow operations remain P15/P16 operations.

---

# 49. `CopyValueOp`

Input:

```text
copyable SSA value
```

Output:

```text
independent copied semantic value
```

Source remains valid.

Mode:

```text
copy-trivial
```

---

# 50. `SemanticCopyValueOp`

Input/output same semantic type.

Requires resolved:

```text
infallible semantic copy plan
```

Source remains valid.

The result receives independent destruction responsibility.

If no such resolved infallible copy exists, the operation is invalid.

---

# 51. `MoveValueOp`

Input:

```text
owned SSA value
```

Output:

```text
same semantic value under new SSA ownership continuation
```

Source becomes unavailable.

No destructor runs for the source.

Destruction responsibility transfers.

---

# 52. `CopyFromPlaceOp`

Reads from an initialized place while preserving source ownership.

Modes:

```text
copy-trivial
copy-semantic-infallible
```

The result becomes a new independent value.

---

# 53. `MoveFromPlaceOp`

Consumes the initialized owned value at a place.

Result:

```text
new owned SSA value
```

Source place state becomes:

```text
moved/uninitialized
```

For a subobject place, aggregate partial state is updated.

---

# 54. `InitializePlaceOp`

Consumes a value into an uninitialized/moved place.

Precondition:

```text
destination does not currently contain a live value requiring destruction
```

After:

```text
destination initialized
new cleanup responsibility exists when required
```

---

# 55. `ReplacePlaceOp`

Consumes the new value into an initialized place and terminates the old
destination ownership exactly once.

Semantic effects:

```text
destroy old value according to plan
install new value
register new ownership/destruction responsibility
```

The RHS is already fully evaluated before this op.

It is not the future source `replace(...)` operation that returns the old value.

---

# 56. `DiscardValueOp`

Consumes a value because its remaining lifetime/result is intentionally unused.

Required provenance:

```text
explicit
implicit-call-result
compiler-temporary
```

For non-trivial types it invokes the canonical destruction plan.

For trivial types it may lower to no machine instruction.

---

# 57. `DestroyValueOp`

Terminal ownership action for an owned SSA value.

Required:

```text
DestructionPlanID
cause
```

Allowed causes include:

```text
scope-exit
function-exit
temporary-end
cleanup
compiler-generated
```

After destroy the value is unavailable.

---

# 58. `DestroyPlaceOp`

Destroys the currently initialized value at an owned place.

Afterward the place is uninitialized.

It is valid only when no ownership has already moved out except explicitly
tracked remaining subobjects.

---

# 59. Partial move state representation

Semantic IR function analysis retains subobject state keyed by:

```text
root storage/place identity
field/variant path
```

No runtime bitmask is required.

For MLIR, explicit `sec.own.move_from_place` plus place identity and verifier
state are sufficient.

---

# 60. Aggregate constructor integration

P17 generalizes constructor transfer actions.

P13 struct construction fields may use:

```text
construct-direct
copy-trivial
copy-semantic-infallible
move
```

P11 union payload construction may use the same applicable actions.

Result/Option payload construction may copy/move according to Sema.

---

# 61. Fixed-array construction integration

Ordinary explicit fixed-array literal element segments may use:

```text
construct-direct
copy-trivial
copy-semantic-infallible
move
```

according to element source/context.

Fixed-array spread keeps its source spread rule:

```text
copy only
```

and may use semantic copy only if Sema explicitly classifies the spread as
infallible implicit semantic copy.

P17 does not turn spread into move.

---

# 62. Non-trivial aggregate destruction unlocked

P17 removes the temporary P13/P14 gate that required whole structs/fixed arrays
to be trivially destructible for ordinary ownership.

A non-trivial aggregate is allowed in the new IR path when:

```text
all transfer actions are represented
a complete destruction plan exists
cleanup registration is represented
no still-deferred operation is required
```

---

# 63. P12 move-only match payloads

P17 enables supported by-value move-only payload bindings.

On a proven variant path:

```text
derive payload place
MoveFromPlaceOp
bind moved SSA value
update container partial state
```

Borrowed payload bindings continue to use P15.

---

# 64. P13 non-trivial field replacement

A mutable stored field can now use:

```text
ReplacePlaceOp
```

instead of the trivial whole-struct `StructReplaceFieldOp` path when the place is
addressable and ownership analysis permits replacement.

The whole owning struct remains owned.

---

# 65. P14 fixed-array ownership boundary after P17

Whole fixed arrays may:

```text
move
be destroyed non-trivially
contain move-only elements
```

P17 still does not permit arbitrary element move-out through runtime indexing.

---

# 66. P15/P16 reference and slice ownership

Reference/slice values retain their established classifications:

```text
ref T          CopyTrivial
ref mut T      MoveOnly
ref T[]        CopyTrivial
ref mut T[]    MoveOnly
```

P17 routes discard/lifetime end correctly without destroying referents.

---

# 67. Function argument transfer

Every by-value call argument has explicit ownership action.

Required actions:

```text
copy-trivial
copy-semantic-infallible
move
construct-direct
```

Borrowed parameters continue to receive reference values.

Caller cleanup responsibility is removed only for consuming/move transfer.

---

# 68. Return transfer

Return is a consuming context for an owned local result.

The returned ownership is established before cleanup.

For trivially copyable machine immediates, physical forwarding/copy is an
optimization detail.

Semantic return ownership remains explicit.

---

# 69. Branch transfer

Owned values crossing CFG edges must preserve ownership state.

Semantic IR branch arguments carry ownership actions.

P17 uses:

```text
move
copy
non-owning
```

classification per branch operand.

Mutually exclusive successor edges may each carry the same current owner because
only one edge executes.

No merge may create two simultaneous owners.

---

# 70. Ownership merge

At a block merge:

```text
all reachable incoming ownership states must be compatible
```

The block argument becomes the owner of the selected incoming transferred value.

Reject:

```text
owned on only some paths without explicit source type state
incompatible partial-move masks
incompatible active variants
incompatible destruction plans
```

---

# 71. Cleanup registration operations

Semantic IR adds:

```text
CleanupTrackOwnedOp
CleanupCancelOp
CleanupDeferRegisterOp
CleanupRunScopeOp
CleanupRunFunctionOp
```

These are high-level compile-time/runtime-semantic operations.

They do not imply a global runtime manager.

---

# 72. `CleanupTrackOwnedOp`

Emitted after successful initialization of an owned value/place requiring
non-trivial cleanup.

Records:

```text
cleanup action ID
scope ID
place/value identity
destruction plan
registration order
```

---

# 73. `CleanupCancelOp`

Cancels one active automatic destruction responsibility after:

```text
move out
early discard/destroy
ownership transfer
replacement of the old incarnation
```

Cancellation itself has no runtime resource-release effect.

---

# 74. `CleanupDeferRegisterOp`

Represents runtime semantic registration when execution reaches a defer.

Contains/references the defer body and capture places.

Captures are:

```text
binding/place references
```

not copied values.

A dynamic loop may execute the registration multiple times.

---

# 75. `CleanupRunScopeOp`

Executes/removes automatic cleanup actions for lexical scopes exited by the
edge.

It does not execute function-scoped defer entries.

Used for:

```text
normal lexical scope exit
break
continue
branch-local cleanup
```

---

# 76. `CleanupRunFunctionOp`

Executes all remaining active function cleanup entries in reverse registration
order.

Used before:

```text
explicit return
implicit void return
Result Err propagation
other normal language-controlled function exit
```

The exit payload has already been ownership-transferred out of local cleanup.

---

# 77. Defer body representation

Recommended Semantic IR:

```text
CleanupDeferRegion
```

with explicit capture-place parameters.

The body is analyzed ordinary Semantic IR except for the existing defer
restrictions.

It returns only to cleanup execution.

It cannot return from the surrounding function or propagate Result errors.

---

# 78. Dynamic defer storage boundary

P17 does not choose the physical representation for a function with an
unbounded number of dynamic defer registrations.

A later cleanup-expansion pass may use:

```text
generated stack storage
static CFG expansion
bounded static slots
other runtime-free local representation
```

No global runtime is required.

---

# 79. Cleanup-plan Sema API

Add:

```go
type ResolvedCleanupPlan struct {
    Function           FunctionID
    Scopes             []ResolvedCleanupScope
    Registrations      []ResolvedCleanupRegistration
    ExitEdges          []ResolvedCleanupExit
    DeferBodies        []ResolvedDeferPlan
}
```

It is read-only after successful analysis.

---

# 80. `ResolvedCleanupRegistration`

Recommended fields:

```text
ActionID
kind
registration source position/order
scope ID
value/place identity
DestructionPlanID
DeferID
capture places
conditional/dynamic registration facts
```

---

# 81. Cleanup exit plan

Each normal exit edge identifies:

```text
exit kind
scopes exited
automatic actions eligible
whether function defers run
protected transferred values
destination block/function return
```

The builder does not rediscover scope cleanup.

---

# 82. Unified cleanup order verifier

The verifier checks that active cleanup actions execute in:

```text
reverse semantic registration order
```

subject to eligibility.

It verifies both:

```text
automatic destruction
defer
```

inside one ordering model.

---

# 83. Trivial destruction optimization

Trivially destructible values need no runtime destroy operation.

However ownership state still changes at:

```text
move
discard
scope end
```

The verifier may omit explicit `DestroyValueOp` only when the type is proven
trivially destructible.

---

# 84. Destruction cause metadata

Keep at least:

```text
scope-exit
function-exit
return-cleanup
error-propagation-cleanup
break-cleanup
continue-cleanup
explicit-discard
implicit-discard
temporary-end
replacement
```

for debugging/verifier diagnostics.

---

# 85. No cleanup on panic by default

P17 does not model C++/Rust-style unwinding.

A panic path may skip pending cleanup.

Do not route `sec.fail.*` through normal function cleanup merely because cleanup
exists.

If panic unwinding is specified later, it requires a new normative package.

---

# 86. MLIR schema version 13

Compiler-generated high-level Sec MLIR uses:

```mlir
sec.dialect_version = 13 : i32
```

Schema versions 1 through 12 remain regression inputs.

Schema v13 adds:

```text
sec.own.copy
sec.own.semantic_copy
sec.own.move
sec.own.copy_from_place
sec.own.move_from_place
sec.own.initialize_place
sec.own.replace_place
sec.own.discard
sec.own.destroy_value
sec.own.destroy_place

sec.cleanup.track_owned
sec.cleanup.cancel
sec.cleanup.defer_register
sec.cleanup.run_scope
sec.cleanup.run_function
```

and ownership/cleanup metadata on existing constructors/calls/branches/returns.

---

# 87. No ownership token MLIR type

P17 does not introduce a runtime or SSA-visible source ownership-token type.

Why:

```text
SSA value identity already identifies value continuations
Place/Storage identity identifies addressable ownership
explicit own.* operations identify transfers
cleanup operations identify destruction responsibility
```

An implementation may use internal analysis tokens.

They are not part of Sec source type identity.

---

# 88. `sec.own.copy`

```text
T -> T
```

Required:

```text
copy_kind = "trivial"
```

Source remains valid.

---

# 89. `sec.own.semantic_copy`

```text
T -> T
```

Required:

```text
resolved copy plan
copy_kind = "semantic-infallible"
```

Source remains valid.

Result has independent destruction responsibility.

---

# 90. `sec.own.move`

```text
T -> T
```

The source SSA ownership is consumed.

The result is the only ownership continuation on that path.

---

# 91. `sec.own.copy_from_place`

Operand:

```text
initialized Place<T>
```

Result:

```text
T
```

Mode:

```text
trivial
semantic-infallible
```

Source place remains initialized.

---

# 92. `sec.own.move_from_place`

Operand:

```text
owned initialized Place<T>
```

Result:

```text
T
```

Source place/subobject becomes moved/uninitialized.

The operation carries subobject/place identity for partial-state verification.

---

# 93. `sec.own.initialize_place`

Operands:

```text
uninitialized Place<T>
owned T
```

No result required.

Destination becomes initialized.

The source ownership transfers to destination.

---

# 94. `sec.own.replace_place`

Operands:

```text
initialized writable Place<T>
owned replacement T
```

Semantically:

```text
destroy old destination
initialize new destination
```

The operation carries the old destruction plan and new cleanup registration
identity.

---

# 95. `sec.own.discard`

Operand:

```text
owned value or reference value
```

Required:

```text
discard_origin
destruction plan when owning non-trivial
```

Reference/slice discard must integrate with borrow end semantics.

---

# 96. `sec.own.destroy_value`

Operand:

```text
owned T
```

No results.

Consumes ownership.

Required:

```text
destruction_plan
cause
```

---

# 97. `sec.own.destroy_place`

Operand:

```text
owned initialized Place<T>
```

No results.

Destination becomes uninitialized/destroyed.

---

# 98. `sec.cleanup.track_owned`

No user-visible result.

Tracks one active destruction responsibility.

Required:

```text
cleanup_action_id
cleanup_scope_id
destruction_plan
registration_ordinal
```

Input is the current value/place identity.

---

# 99. `sec.cleanup.cancel`

Required:

```text
cleanup_action_id
reason
```

Reason:

```text
move
transfer
discard
early-destroy
replacement
```

---

# 100. `sec.cleanup.defer_register`

High-level dynamic registration operation with one defer body region.

Capture operands are P15 Place values or equivalent binding-place handles.

Required:

```text
defer_id
registration site identity
function cleanup scope
```

The operation may execute multiple times.

---

# 101. `sec.cleanup.run_scope`

Required:

```text
scope IDs being exited
exit kind
```

Executes eligible automatic cleanup in canonical order.

Does not execute function-scoped defer entries.

---

# 102. `sec.cleanup.run_function`

Required:

```text
exit kind
```

Executes all remaining active cleanup entries in reverse registration order,
including defer entries.

Normal exit only.

---

# 103. Cleanup operation physical status

Schema v13 cleanup operations are still high-level Sec semantics.

They are not LLVM EH.

They are not a global runtime cleanup stack.

They remain until a later cleanup expansion/physical lowering stage.

---

# 104. Existing operation ownership metadata

Add/standardize discardable or custom attrs on operations that carry ownership.

Examples:

```text
sec.ownership_action
sec.argument_ownership_actions
sec.result_ownership_action
sec.branch_ownership_actions
sec.return_ownership_action
sec.cleanup_action_id
sec.destruction_plan
```

Exact TableGen representation may use typed attrs.

Do not use free-form strings where a bounded enum/custom attr is practical.

---

# 105. Constructor ownership metadata

Update:

```text
sec.struct.construct
sec.array.construct
sec.union.construct
sec.result.ok
sec.result.err
```

to permit P17 actions:

```text
construct-direct
copy-trivial
copy-semantic-infallible
move
```

subject to source operation semantics.

---

# 106. P13 struct integration

Remove the P13 implementation rejection for a struct merely because it is
non-trivially destructible.

Keep explicit rejections for features still lacking a represented semantic
operation.

Struct field construction now adopts/moves/copies ownership according to the
resolved action.

---

# 107. P14 fixed-array integration

Whole fixed arrays can be non-trivial owned values.

`sec.array.default` may represent a non-trivial default when the resolved
default construction is infallible and destruction plan exists.

Large destruction stays compact/high-level.

---

# 108. P15 reference integration

P17 does not destroy referents for reference destruction/discard.

`ref mut` move remains explicit and move-only.

Borrow end and ownership end remain distinct semantic concepts even if they
occur at the same source point.

---

# 109. P16 slice integration

Shared slice copy and mutable slice move retain P16 operations or may
canonicalize through P17 ownership interfaces.

No element ownership is transferred by slice copy/move.

---

# 110. Result/error propagation cleanup

Before propagating Err:

```text
Err payload ownership is established
local ownership of the propagated Result/error is transferred out
remaining cleanup runs
return occurs
```

The propagated error is not destroyed by callee cleanup.

---

# 111. Match branch cleanup

Branch-local owning values are cleaned when leaving the branch unless moved into:

```text
merge result
outer storage
return
call
other consuming continuation
```

P12 match CFG receives explicit cleanup before branch exit.

---

# 112. Loop cleanup

P17 requires explicit cleanup plans for:

```text
iteration local scope end
continue
break
function return from loop
```

Function-scoped defers registered in the loop do not run on iteration cleanup.

---

# 113. Defer-required values and moves

If an active registered defer requires a binding/place:

```text
Sema prevents a conflicting move/destruction before defer execution
```

P17 cleanup verifier checks the emitted IR does not consume it anyway.

---

# 114. Destruction verifier

Register:

```bash
--sec-verify-ownership
```

It checks:

```text
copyability
move source availability
no use after move
no ref-mut copy
partial subobject state
initialization/reinitialization
branch ownership compatibility
argument/return transfer
exactly one terminal ownership action for non-trivial owned values
```

---

# 115. Cleanup verifier

Register:

```bash
--sec-verify-cleanups
```

It checks:

```text
cleanup registration after successful initialization
cleanup cancellation after transfer/early destruction
no destroy of moved value
no destroy of uninitialized subobject
all remaining initialized owned values cleaned on normal exits
reverse registration order
defer registration/execution semantics
return/error payload excluded from cleanup
scope versus function cleanup eligibility
no normal cleanup inserted after non-cleanup termination
```

---

# 116. Destruction-plan verifier

Register:

```bash
--sec-verify-destruction-plans
```

It checks:

```text
every non-trivial concrete type has one resolved plan
struct field plan completeness/order
array element plan
union/Result/Option active payload coverage
custom-free restrictions
no recursive same-value free
allocator/deallocator pairing metadata where applicable
```

---

# 117. No second borrow checker

P17 ownership verification consumes P15 borrow/place facts.

It does not reimplement source borrow analysis.

Ownership and borrow states must agree.

---

# 118. Optimization rules

Permitted after verification:

```text
copy elision
move elision
direct destination construction
RVO/NRVO
trivial destroy elimination
identical cleanup merge
static cleanup expansion
```

Only when semantic ownership/destruction behavior remains identical.

---

# 119. Forbidden optimization effects

Optimization must not:

```text
create second owner
revive moved source
skip non-trivial destruction
double destroy
change observable destruction order
change defer order
destroy returned/transferred value
destroy moved field
copy move-only ownership
introduce hidden allocation
```

---

# 120. No LLVM decision-making

LLVM lowering may not decide:

```text
copy versus move
whether a destructor exists
which branch needs cleanup
which fields were moved
defer registration/execution order
whether an argument consumed ownership
```

All of those are fixed before LLVM translation.

---

# 121. No mandatory runtime

P17 does not require:

```text
GC
reference counting
global ownership table
global cleanup registry
runtime borrow tracker
exception unwinder
```

Dynamic defer registration may use compiler-generated local storage in a later
lowering.

---

# 122. Required Sema tests

```text
copy trivial local
move-only local move
use after move
copy semantic infallible plan
fallible clone remains call
move-only call argument
move-only return
whole-value reinitialization
transactional fallible replacement
partial struct field move
partial field reinitialization
custom-free partial move rejected
union/Result/Option payload move under variant proof
array element partial move remains unsupported
defer prevents early move
```

---

# 123. Required destruction-plan tests

```text
trivial scalar
struct reverse field order
partially moved struct skips field
fixed array reverse index plan
zero-length array
union active payload
Result active payload
Option Some/None
ref/ref-mut trivial referent-independent
slice trivial referent-independent
nested aggregate plans
```

---

# 124. Required discard tests

```text
explicit trivial binding discard
explicit move-only resource discard
implicit ordinary call-result discard
Result explicit discard
Option active payload discard
reference discard ends borrow but not referent
ref-mut discard releases holder
discarded binding unavailable
partial direct-field discard where permitted
```

---

# 125. Required replacement tests

```text
initialized trivial replace
initialized non-trivial replace
RHS before old destruction
fallible RHS failure keeps old destination
move-only replacement
reinitialize moved local without old destroy
field replacement
alias/borrow conflict remains Sema error
self-assignment move-only rejected
```

---

# 126. Required cleanup-order tests

```text
locals reverse successful initialization
struct fields reverse declaration
array elements reverse index
defer LIFO
interleaved local/defer unified registration order
conditional defer registration
multiple return paths
Result propagation
branch-local cleanup
break cleanup
continue cleanup
loop iteration locals
```

---

# 127. Required defer tests

```text
defer reads current binding value at execution
defer does not value-copy at registration
defer-required value cannot move early
conditional defer registers conditionally
loop defer registers per execution
loop registrations execute reverse registration order
return expression evaluated before defer
propagated Err prepared before defer
defer cannot replace return/error
no normal cleanup on aborting panic path
```

---

# 128. Required Semantic IR tests

```text
CopyValueOp
SemanticCopyValueOp
MoveValueOp
CopyFromPlaceOp
MoveFromPlaceOp
InitializePlaceOp
ReplacePlaceOp
DiscardValueOp
DestroyValueOp
DestroyPlaceOp
cleanup track/cancel
defer register region
scope cleanup
function cleanup
partial struct state
variant payload state
ownership branch merge
```

---

# 129. Required dialect tests

Schema v13:

```text
all sec.own.* ops parse/print/verify
all sec.cleanup.* ops parse/print/verify
constructor ownership attrs
call argument ownership attrs
return ownership attrs
branch ownership attrs
defer region captures Places
schema-v12 regressions
```

---

# 130. Required ownership verifier negative tests

```text
copy move-only type
semantic copy without plan
move already moved value
use after move
double consume
double destroy
destroy moved value
destroy uninitialized field
ref-mut copy
partial move from custom-free type
incompatible ownership merge
return value also cleaned
call-consumed value also cleaned
```

---

# 131. Required cleanup verifier negative tests

```text
missing cleanup registration
missing normal-exit cleanup
wrong destruction order
wrong defer order
uncancelled moved cleanup
cleanup canceled twice
defer executed on lexical block exit
defer omitted on normal function return
defer after returned control
panic path incorrectly claims normal cleanup
```

---

# 132. Required P11-P16 integration tests

P11:

```text
move-only union payload construction/destruction
active payload move
```

P12:

```text
move-only match payload binding
borrowed payload unchanged
branch cleanup
```

P13:

```text
non-trivial struct local
move-only field construction
field move/reinit
non-trivial field replacement
```

P14:

```text
fixed array of non-trivial elements
whole array move
reverse destruction
```

P15:

```text
reference discard/end-borrow
no referent destruction
```

P16:

```text
shared slice copy
mutable slice move
slice destruction only ends holder
```

---

# 133. End-to-end source examples

Required:

```text
move-only local transferred to another local
move-only parameter consumption
move-only return
resource discarded explicitly
ordinary non-trivial temporary implicitly discarded
non-trivial struct scope cleanup
non-trivial fixed-array scope cleanup
partial struct field move
field reinitialization
Result payload move
Option payload move
try propagation with cleanup
return with interleaved defer/destruction
loop break/continue cleanup
```

No hand editing of generated IR.

---

# 134. Explicitly deferred

P17 does not implement:

```text
user-defined free syntax acceptance
fallible implicit copy
arbitrary fixed-array element move-out
dynamic owning T[]
physical allocator/deallocator lowering
stable/weak handles
physical cleanup-stack representation
panic unwinding
general interface dynamic destruction representation if not already canonical
FFI ownership transfer beyond existing declared-contract metadata
source replace/take/swap intrinsics
physical memcpy/memmove selection
LLVM destruction lowering
```

---

# 135. Architecture rules

Non-negotiable:

```text
Every copy and move is explicit in Semantic IR.

Pure move cannot fail.

Implicit semantic copy cannot hide failure.

Move transfers destruction responsibility.

Moved source is unavailable.

No moved value is destroyed.

No initialized owned value is forgotten on a normal exit.

Replacement evaluates new value before terminating old ownership.

Reinitialization of moved storage does not destroy a nonexistent old value.

Partial struct/payload state is explicit.

Array element partial move remains deferred.

Discard is ownership consumption, not unused SSA.

Discard and scope cleanup use the same destruction engine.

References/slices never destroy referents.

Cleanup uses one deterministic registration order.

Successful initialization and executed defer are cleanup registration events.

Cleanup executes in reverse registration order.

Defer captures binding/place identity, not a value copy.

Defer is function-scoped.

Return/error ownership is established before cleanup.

Panic does not imply unwinding.

Custom-free source syntax remains restricted until separately implemented.

Ownership state is compile-time semantic information, not a runtime ownership table.

No lower stage infers copy/move/destruction from physical instructions.

No mandatory runtime is introduced.

No LLVM dialect is generated by P17.
```

---

# 136. Acceptance criteria

Package 17 is complete only when:

```text
[ ] baseline documents repo 152c772 + local P13-P16 or newer equivalent
[ ] previous package regressions remain green
[ ] ownership/destruction/defer/discard synchronization applied
[ ] Semantic IR ownership amendment applied
[ ] schema-v13 dialect rulebook installed
[ ] lowering-v13 rulebook installed
[ ] copy classification is complete for concrete runtime types
[ ] destruction classification is complete for concrete runtime types
[ ] ResolvedTransferPlan API implemented/read-only
[ ] ResolvedDestructionPlan API implemented/read-only
[ ] ResolvedCleanupPlan API implemented/read-only
[ ] explicit trivial copy IR implemented
[ ] explicit infallible semantic copy IR implemented
[ ] explicit move IR implemented
[ ] copy/move from Place implemented
[ ] initialization/reinitialization implemented
[ ] replacement implemented
[ ] partial struct field move implemented
[ ] field reinitialization implemented
[ ] active variant payload move implemented
[ ] arbitrary fixed-array element partial move remains rejected
[ ] explicit/implicit discard IR implemented
[ ] non-trivial destruction IR implemented
[ ] struct/array/union/Result/Option destruction plans implemented
[ ] cleanup tracking/cancellation implemented
[ ] unified cleanup registration order implemented
[ ] defer registration region implemented
[ ] defer captures Place/binding identity
[ ] scope cleanup implemented
[ ] function-exit cleanup implemented
[ ] return/error payload excluded from cleanup
[ ] no cleanup-on-panic assumption introduced
[ ] schema-v13 own/cleanup ops implemented
[ ] ownership metadata added to constructors/calls/branches/returns
[ ] --sec-verify-ownership registered
[ ] --sec-verify-cleanups registered
[ ] --sec-verify-destruction-plans registered
[ ] P11-P16 non-trivial integrations implemented where specified
[ ] user-defined free syntax remains gated
[ ] no mandatory runtime
[ ] no physical cleanup representation selected
[ ] no LLVM ownership/destruction decision deferred
[ ] check-sec-mlir passes
[ ] go test ./... passes
[ ] legacy paths remain operational
```

---

# 137. Required implementation report

Codex must report:

```text
1. repository HEAD implemented against
2. local/merged P13-P16 status
3. previous package status
4. ownership/destruction/defer/discard synchronization
5. files added
6. files modified
7. copy classification implementation
8. destruction classification implementation
9. ResolvedTransferPlan API
10. ResolvedDestructionPlan API
11. ResolvedCleanupPlan API
12. whole-value copy implementation
13. semantic-copy implementation
14. whole-value move implementation
15. place copy/move implementation
16. initialization/reinitialization
17. replacement algorithm
18. partial struct move/reinit
19. union/Result/Option payload move
20. discard implementation
21. destruction plan lowering
22. struct/array/variant destruction order
23. cleanup track/cancel
24. defer registration/capture representation
25. scope cleanup
26. function cleanup
27. error propagation cleanup
28. branch/loop cleanup
29. schema-v13 ops/attrs
30. constructor/call/branch/return ownership metadata
31. ownership verifier
32. cleanup verifier
33. destruction-plan verifier
34. P11-P16 integration changes
35. wide-type regression tests
36. non-trivial aggregate tests
37. defer/order tests
38. unsupported partial-array/custom-free tests
39. CMake commands
40. exact LLVM/MLIR version
41. check-sec-mlir result
42. go test ./... result
43. end-to-end source -> schema-v13 results
44. deviations
45. recommendations for Package 18
```

---

# 138. Package 18 boundary

Recommended Package 18:

```text
Owning Dynamic Array Semantic Value Representation
```

P17 provides the ownership/destruction substrate needed for owning `T[]`.

Recommended scope:

```text
owning T[] type
length/capacity
allocation identity
allocator/deallocator pairing
empty owning array
construction
reserve/growth semantics
relocation class
element initialization
push/append ownership transfer
index Place integration
array-to-slice borrowing
move-only descriptor ownership
destruction of initialized elements in reverse order
backing storage release
generation/epoch invalidation on relocation/release
high-level !sec.dynamic_array
no physical allocator ABI yet
```

Package 18 should reuse P15/P16 reference invalidation identities rather than
creating a collection-specific reference model.
