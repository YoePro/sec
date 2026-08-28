# Sec MLIR Program - Implementation Package 19

## Package status

Implementation package for the Sec compiler.

Package ID: `SEC-MLIR-P19`  
Package title: `Allocation Context and Arena Semantic Lowering`  
Repository: `https://github.com/YoePro/sec`  
Repository branch: `main`  
Repository sync commit used for this package: `152c772`  
Local predecessors: `SEC-MLIR-P13` through `SEC-MLIR-P18`  
Repository sync date: `2026-08-10`  
Semantic IR version before package: `1`  
Semantic IR version after package: `1`  
Sec MLIR dialect schema before package: `14`  
Sec MLIR dialect schema after package: `15`  
Sec MLIR lowering specification before package: `14`  
Sec MLIR lowering specification after package: `15`

Package 19 makes Sec 0.1 allocation-context and Arena semantics explicit in the
new Semantic IR and Sec MLIR pipeline.

It implements the semantic core for:

```text
Arena builtin recognition
move-only Arena ownership
ArenaDomain identity
Arena state-version tracking
validity epochs
borrowed fixed Arenas
owned fixed Arenas
growable non-relocating Arenas
typed Arena.New[T]
typed Arena.Alloc[T]
AllocationError flow
atomic allocation failure
Arena.Reset
Arena.Release
implicit Arena destruction
nested Arena dependencies
allocation-context selection and propagation
no-allocation profiles
ordered Arena effects
basic Arena capacity-demand summaries
P18 dynamic-array allocation-context integration
```

It does not choose a universal Arena descriptor, allocator ABI, heap, runtime
manager, or final provider implementation.

---

# 1. Normative authority

Implementation follows:

```text
rules/memory/arena.md
rules/memory/allocation.txt
rules/memory/storage.md
rules/memory/reference_model.md
rules/memory/layout.md
rules/types/default_values.md
rules/library/core-library.md
rules/analysis/effect_analysis.md
rules/analysis/call_graph.md
rules/memory/ownership.md
rules/memory/borrowing.md
rules/memory/destruction.md
rules/errors/panic.md
    ↓
local P13-P18 normative amendments
    ↓
rules/compiler/semantic_ir.txt
rules/mlir/sec_mlir.md
rules/mlir/sec_mlir_dialect.md
rules/mlir/sec_mlir_lowering.md
    ↓
implementation package
    ↓
implementation
```

`rules/memory/arena.md` is canonical for Arena-specific Sec 0.1 semantics.

Before implementation:

1. apply `sec_arena_allocation_sync_package19.md`;
2. apply `sec_semantic_ir_arena_package19.md` to `rules/compiler/semantic_ir.txt`;
3. update `rules/mlir/sec_mlir_dialect.md` with
   `sec_mlir_dialect_package19.md`;
4. update `rules/mlir/sec_mlir_lowering.md` with
   `sec_mlir_lowering_package19.md`.

No new source syntax is introduced.

---

# 2. Repository and local predecessor rule

GitHub `main` remains:

```text
152c772
```

P16 and P17 have been obtained locally without a newer GitHub synchronization.
P18 is also a local package built on the same repository baseline.

P19 therefore uses:

```text
GitHub 152c772
+
local P13-P18 package semantics
```

If a newer HEAD contains those packages before implementation, Codex must report
the new HEAD and verify that the relevant semantics are unchanged.

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

P19 does not change their status.

They may be allocated through Arena APIs when all Arena type requirements are
satisfied.

---

# 4. Arena is a semantic builtin

`Arena` is a compiler-known semantic builtin type.

It does not require an ordinary source declaration.

The compiler owns:

```text
member existence
type checking
ownership behavior
ArenaDomain identity
allocation effects
Semantic IR meaning
lowering contracts
```

Core/target code may provide helpers.

---

# 5. Lowercase `arena` is not a keyword

The lowercase name:

```sec
arena
```

is an ordinary identifier.

P19 must not preserve or introduce a reserved lowercase `arena` keyword merely
because the builtin type is `Arena`.

---

# 6. Arena ownership

Canonical:

```text
Arena -> MoveOnly
```

Arena is non-copyable.

Moving an Arena:

```text
transfers the owner
preserves the same ArenaDomain
preserves the current validity epoch
preserves backing state
preserves provider state
does not invalidate existing Arena-backed references
```

P17 owns the general move semantics.

---

# 7. ArenaDomain identity

Add/reuse:

```go
type ArenaDomainID uint32
```

An ArenaDomain:

```text
is compiler-visible
is independent of physical address
belongs to one live Arena owner
has one current validity epoch
may depend on another storage domain
```

ArenaDomain identity is not source type identity.

---

# 8. Arena owner state versus validity epoch

These are separate concepts.

Recommended:

```go
type ArenaStateVersionID uint32
```

Rules:

```text
ordinary successful allocation:
    Arena state version changes
    validity epoch does not change

successful growth:
    Arena state version changes
    validity epoch does not change

Reset:
    Arena state version changes
    validity epoch advances

Release:
    Arena owner is consumed
    ArenaDomain ends
```

The backend must not conflate state version with epoch.

---

# 9. Arena backing kinds

Canonical Arena backing ownership kinds:

```text
Owned
Borrowed
Static
TargetProvided
```

Growth policy is independent:

```text
Fixed
Growable
```

A growable Arena may contain multiple stable segments.

---

# 10. Arena backing maps to canonical storage facts

Arena does not introduce a second global storage taxonomy.

Arena facts map to existing P15/P18/storage concepts:

```text
StorageOrigin = Arena
BackingRelation
ReclamationAuthority
AddressStability
MemorySpace
RegionDependencies
InvalidationDomain
ValidityEpoch
```

Examples:

```text
borrowed buffer Arena:
    Arena controls ArenaDomain
    physical backing owned elsewhere
    backing relation = Borrowed
    reclamation authority = ExternalOwner or enclosing storage owner

owned fixed Arena:
    Arena controls ArenaDomain
    backing acquired from resolved provider
    reclamation authority follows provider contract

static Arena:
    Arena controls ArenaDomain
    physical bytes are static
    no individual deallocation

target-provided Arena:
    Arena controls ArenaDomain
    release follows target provider contract
```

---

# 11. Borrowed fixed Arena

Canonical source:

```sec
let mut arena := Arena.FromBuffer(ref mut buffer)
```

Conceptual signature:

```sec
fn FromBuffer(buffer: ref mut byte[]) Arena
```

P19 supports this end-to-end.

---

# 12. `Arena.FromBuffer` semantics

`FromBuffer`:

```text
creates a fresh ArenaDomain
creates initial live epoch
takes/retains exclusive borrow of the complete supplied mutable slice
uses the slice as fixed backing
does not allocate
does not grow
is infallible
does not deallocate the backing owner on Release
```

The returned Arena owns the ArenaDomain, not the backing bytes.

---

# 13. Borrowed-backing lifetime

The supplied mutable slice backing must remain valid for the complete Arena
lifetime.

While the Arena is live:

```text
conflicting direct access to the backing slice is invalid
parent owner may not invalidate/reclaim the backing
```

Release ends the retained backing borrow.

---

# 14. Empty borrowed Arena

A zero-length mutable buffer may create a valid Arena.

It:

```text
has zero capacity
may Reset
may Release
can satisfy zero-element allocation
cannot satisfy positive allocation in a fixed policy
```

No special nullable Arena exists.

---

# 15. Owned fixed Arena

Canonical source:

```sec
let mut arena := try Arena.WithCapacity(4096)
```

Conceptual source type:

```sec
fn WithCapacity(capacity: uint) Result[Arena, AllocationError]
```

P19 supports this end-to-end when the CompilationPlan has a compatible provider.

---

# 16. `Arena.WithCapacity` semantics

Successful construction:

```text
creates fresh ArenaDomain
acquires owned fixed backing through resolved provider
establishes initial epoch
sets used capacity to zero
sets fixed capacity
records provider/reclamation plan
registers P17 Arena destruction responsibility
```

Failure:

```text
returns AllocationError
creates no live ArenaDomain
publishes no partial Arena owner
```

---

# 17. Growable Arena semantic model

P19 implements the high-level growable Arena model.

Canonical source when the constructor is available:

```sec
let mut arena := try Arena.Growable(4096)
```

Conceptual type:

```sec
fn Growable(initialCapacity: uint) Result[Arena, AllocationError]
```

The public constructor may remain frontend/profile-gated if the current
implementation has not enabled it.

The IR model must still support growable compiler-managed or target-provided
allocation contexts.

---

# 18. Growable Arena cannot relocate live allocations

A growable Arena may acquire only storage that preserves all prior live
allocation addresses and bounds.

Allowed strategies include:

```text
additional stable segments
reserved virtual address space
target-provided non-relocating extension
other profile-defined stable strategy
```

Forbidden:

```text
allocate larger buffer
copy old allocations
change old addresses
continue using prior direct references
```

Growth does not advance the Arena epoch because earlier allocations remain valid.

---

# 19. Arena capacity units

Arena capacity is measured in bytes.

`Arena.Alloc[T](count)` interprets `count` as number of `T` elements.

Required calculations are semantic checked arithmetic:

```text
alignedOffset = AlignUp(currentOffset, AlignOf(T))
payloadSize   = CheckedMultiply(count, SizeOf(T))
endOffset     = CheckedAdd(alignedOffset, payloadSize)
```

No host-width truncation.

---

# 20. Arena layout source

`SizeOf(T)` and `AlignOf(T)` come from the active CompilationPlan's canonical
resolved layout.

P19 must not independently invent layout.

---

# 21. Arena safe typed allocation restrictions

Safe initial:

```text
Arena.New[T]
Arena.Alloc[T]
```

require:

```text
T complete layout
T sized
known alignment
valid compiler-defined default
safe initialization
T trivially destructible
```

P17's support for non-trivial destruction does not broaden this Arena source
rule.

---

# 22. Why Arena allocation remains trivially destructible

Arena Reset/Release perform bulk storage reclamation.

The Arena is not an implicit destructor registry for arbitrary allocated
objects.

Owning containers may use Arena backing only when the container itself tracks
and destroys its elements before reset/release.

---

# 23. `Arena.New[T]`

Canonical:

```sec
let value := try arena.New[Value]()
```

Conceptual source contract:

```sec
fn New[T]() Result[ref mut T, AllocationError]
```

The implicit Arena receiver is mutated only for the duration of the operation.

---

# 24. `Arena.New[T]` semantics

Atomic operation:

```text
validate Arena live state
validate T/layout/alignment/default
compute aligned range
check capacity or complete stable growth
reserve complete range
fully initialize one T
publish ref mut T
commit next Arena state version
```

Failure:

```text
Arena state unchanged
Arena epoch unchanged
prior allocations valid
no partially initialized T observable
```

---

# 25. `Arena.Alloc[T](count)`

Canonical:

```sec
let values := try arena.Alloc[Value](count)
```

Conceptual source contract:

```sec
fn Alloc[T](count: uint) Result[ref mut T[], AllocationError]
```

---

# 26. `Arena.Alloc[T]` semantics

Atomic operation:

```text
validate Arena/T
compute count * size with checked arithmetic
compute alignment padding
check capacity or acquire complete stable growth
reserve complete range
fully initialize every T
publish ref mut T[] of exactly count elements
commit next Arena state version
```

No shorter successful slice is permitted.

---

# 27. Temporary mutable Arena borrow

`New`/`Alloc` mutate Arena control state.

The temporary exclusive access to the Arena owner/control state ends when the
operation returns.

The returned reference/slice retains:

```text
ArenaDomain dependency
current validity epoch
allocation bounds
storage origin
authority
```

It does not keep `ref mut Arena` live.

Repeated allocation is therefore allowed.

---

# 28. Repeated allocation does not invalidate prior allocation

Ordinary successful Arena allocation:

```text
does not advance epoch
does not invalidate prior allocations
does not reuse prior allocated bytes in current epoch
```

Positive-sized successful allocations in one epoch are non-overlapping.

P19 may expose this to alias analysis.

---

# 29. Zero-element allocation

`Arena.Alloc[T](0)`:

```text
is valid
succeeds
returns valid empty mutable slice
consumes no capacity
requires no growth
creates no dereferenceable element
does not advance epoch
```

The stable source result type remains `Result`.

A proof may eliminate the failure branch without changing source typing.

---

# 30. Zero-sized T

If the canonical layout permits `SizeOf(T) == 0`:

```text
Alloc[T](count) may consume no payload bytes
count semantic elements still exist
element identity/bounds remain
no unique byte address per element is implied
```

P19 does not decide whether user-visible zero-sized types are otherwise allowed.

---

# 31. Full default initialization

Safe Arena typed allocation never exposes uninitialized `T`.

Initialization uses canonical default semantics.

It does not imply all physical bytes are zero.

---

# 32. AllocationError

P19 uses the existing compiler-known/core error:

```sec
enum AllocationError {
    OutOfMemory
    Unsupported
    InvalidSize
    InvalidAlignment
}
```

No Arena-specific error type is added.

---

# 33. Allocation error mapping

Minimum mapping:

```text
provider/capacity unable to satisfy request:
    OutOfMemory

target/profile/provider does not support required operation:
    Unsupported

count/size arithmetic or representable extent invalid:
    InvalidSize

required alignment cannot be satisfied:
    InvalidAlignment
```

Statically impossible source use may be diagnosed at compile time instead of
constructing a guaranteed Err path.

---

# 34. Stable Result contract

`WithCapacity`, `Growable`, `New[T]` and `Alloc[T]` keep their source Result
types even when Sema proves capacity/provider success.

Proof-driven lowering may remove the failure branch.

It must not change callable source type identity.

---

# 35. Allocation atomicity

Arena creation/allocation/growth is atomic from Sec program semantics.

On any failed operation before publication:

```text
ArenaDomain state unchanged or absent as appropriate
epoch unchanged
cursor/state unchanged
existing allocations unchanged
no partial segment published
no partial typed object/slice published
```

---

# 36. Arena Reset

Canonical source:

```sec
arena.Reset()
```

Conceptual type:

```sec
fn Reset() void
```

Reset is mutating and non-consuming.

---

# 37. Reset dependency rule

Reset is valid only when no validity-preserving dependency crosses it.

Blockers include:

```text
owned values stored in Arena
shared refs
mutable refs
slices
nested Arenas
Arena-backed owning containers
closure captures
task/thread captures
deferred operations
foreign-retained dependencies
strong handles
returned/live values
```

P19 uses P15-P18 dependency facts.

---

# 38. Reset uses non-lexical liveness

A lexical binding may remain in scope after its last use.

Reset is permitted once all relevant semantic dependencies are dead.

P19 must not use lexical scope alone as the reset test.

---

# 39. Reset transition

Reset performs:

```text
ArenaDomain remains live
ArenaDomain identity preserved
current allocation epoch ends
all old allocation dependencies become invalid
allocation cursors reset
backing retained
new distinguishable epoch published
Arena state version advances
```

Storage transition:

```text
AdvanceEpoch
```

---

# 40. Reset does not zero storage

Reset does not require clearing backing bytes.

Prior typed object lifetimes have ended.

Future safe typed allocation must initialize new T values before exposure.

Optional zeroing is a target/security/debug strategy only.

---

# 41. Reset atomicity

No code may observe:

```text
partial cursor reset
mixed old/new epoch state
reused storage published under old epoch
```

The high-level reset operation is one atomic semantic transition.

---

# 42. Epoch exhaustion

Epoch advancement is checked.

It must never wrap and revive stale references.

At exhaustion the resolved profile strategy must:

```text
safely rekey/replace the ArenaDomain
or
use deterministic panic/target trap
```

P19 does not invent silent wrap.

Default logical epoch width remains 64 bits from P15/reference model.

---

# 43. Arena Release

Canonical source:

```sec
arena.Release()
```

Conceptual type:

```sec
fn Release() void
```

Release is a consuming terminal operation on the Arena owner.

---

# 44. Release dependency rule

No validity-preserving Arena dependency may cross Release.

Weak/stale-capable future handle identities may survive only according to their
separate fallible resolution contract.

P19 does not implement stable/weak handles.

---

# 45. Release transition

Release:

```text
consumes Arena owner
ends ArenaDomain
invalidates dependencies
releases backing according to backing/provider contract
ends retained borrowed-backing authority
permits no later Arena use
```

Storage transition:

```text
EndDomain
```

followed by provider reclamation only when required.

---

# 46. Release by backing kind

Owned:

```text
return owned backing segments to provider
```

Borrowed:

```text
end exclusive backing borrow
do not deallocate backing owner
```

Static:

```text
end ArenaDomain
do not deallocate static bytes
```

TargetProvided:

```text
follow provider contract
always end ArenaDomain
```

---

# 47. Implicit Arena destruction

Arena has a non-trivial P17 destruction plan:

```text
DestroyArena
```

Normal destruction of a still-owned Arena performs terminal Release semantics.

If explicit Release already consumed it:

```text
no second implicit Release
```

Double release is invalid.

---

# 48. Panic cleanup boundary

Arena follows the active CompilationPlan panic cleanup policy.

P19 does not introduce a universal unwinder.

The current high-level package must preserve enough cleanup ownership metadata
that a later profile-specific panic lowering can:

```text
release owned Arena during cleanup-capable panic
or
skip guaranteed release for immediate trap/abort
```

Normal P17 cleanup remains fully implemented.

---

# 49. Nested Arena

P19 supports nested Arena dependency semantics.

Canonical construction:

```sec
let childBuffer := try parent.Alloc[byte](4096)
let mut child := Arena.FromBuffer(childBuffer)
```

---

# 50. Nested Arena facts

The child:

```text
has fresh child ArenaDomain
has own child epoch
borrows parent allocation backing
depends on parent ArenaDomain and parent epoch
```

Parent Reset/Release is invalid while child remains live.

Child Release:

```text
ends child domain
ends child borrow
does not individually reclaim parent allocation bytes
```

---

# 51. Allocation context

Every Sec callable invocation has:

```text
zero or one active allocation context
```

The context is compiler-visible semantic state.

It is not:

```text
source global
automatically chosen lexical Arena
mandatory TLS allocator
implicit heap
mandatory runtime object
```

---

# 52. `MayAllocate` versus `RequiresAllocationContext`

These are distinct facts.

`MayAllocate`:

```text
reachable execution may perform allocation
```

`RequiresAllocationContext`:

```text
callable contains/reaches an implicit allocating operation that needs ambient
context
```

A function allocating only through an explicit Arena may be `MayAllocate`
without `RequiresAllocationContext`.

---

# 53. Allocation-context selection order

For an allocating operation:

```text
1. explicit Arena selected by that operation
2. propagated ambient allocation context
3. compiler-managed local Arena with proven backing/non-escape
4. target-provided context
5. no context -> compile-time error
```

This decision is complete before physical lowering.

---

# 54. Lexical Arena values are not ambient candidates

The compiler must not scan in-scope `Arena` variables and guess one.

An explicit Arena becomes relevant only when selected by the operation's source
contract or explicit argument.

---

# 55. Compiler-managed local context

A compiler-local Arena/context is permitted only when:

```text
concrete backing strategy exists
lifetime is proven
non-owning references do not escape
failure semantics stay correct
profile permits it
```

Possible backing includes:

```text
bounded stack storage
static storage
caller-context-backed stable storage
target-provided local storage
```

No hidden escape promotion.

---

# 56. Synchronous propagation

A synchronous Sec call propagates the active context only when the callee's
summary says:

```text
RequiresAllocationContext
```

Functions not requiring it receive no mandatory hidden context.

---

# 57. Internal high-level context parameter

P19 chooses an explicit high-level IR representation:

```text
!sec.alloc_context
```

for internal Sec MLIR only.

A `func.func` that requires ambient allocation gets one compiler-hidden context
argument.

Required argument attribute:

```text
sec.hidden_allocation_context = true
```

or equivalent typed metadata.

This does not modify the source function type.

---

# 58. Internal direct call propagation

A direct Sec-to-Sec call to a function requiring context passes the current
`!sec.alloc_context`.

A call to a function not requiring context does not add one.

The hidden context may later be fully eliminated.

---

# 59. Explicit Arena context

When an API explicitly accepts/selects Arena control for an allocating
operation, the builder may create a temporary compiler context:

```text
AllocationContextFromArenaOp
```

from the resolved Arena authority.

This context is selected because the operation explicitly requested the Arena.

It is never inferred from lexical presence.

---

# 60. Target-provided context

A Sec entry root may receive/create:

```text
TargetAllocationContextOp
```

when the CompilationPlan provides one.

No universal target context is assumed.

---

# 61. No-allocation profile

A profile may provide no ambient dynamic allocation context.

Canonical consequences:

```text
allocating operations needing ambient context -> compile-time error
Arena.FromBuffer may remain available
no hidden heap fallback
no automatic escape promotion
```

Arena-specific allocation guarantees may separately forbid `Arena.New/Alloc`
even with preexisting backing according to profile/effect rules.

---

# 62. `@noAlloc` is distinct from missing ambient context

`@noAlloc` is an effect guarantee.

Arena rulebooks currently classify:

```text
Arena.New
Arena.Alloc
```

as allocation effects even when existing capacity is sufficient.

P19 does not redefine that guarantee.

---

# 63. Spawn/task/thread allocation context boundary

A spawned task/thread is a new execution context.

It does not automatically inherit the parent's mutable Arena context.

P19 records this boundary in callable/context summaries.

Full spawn/await/join dependency lowering remains in the concurrency packages.

---

# 64. Foreign entrypoint boundary

Foreign code does not automatically supply a Sec allocation context.

An exported Sec callable requiring one needs:

```text
generated wrapper selecting target context
explicit Sec-aware ABI contract
or export rejection
```

P19 must not silently add `!sec.alloc_context` to an ordinary foreign ABI.

---

# 65. Arena ordered effects

P19 records ordered Arena effects:

```text
ArenaCreate(domain)
ArenaAllocate(domain)
ArenaReset(domain)
ArenaRelease(domain)
```

and provider/storage effects where applicable.

Order matters.

They are not one unordered boolean.

---

# 66. Operation effect summaries

Minimum:

```text
Arena.FromBuffer:
    ArenaCreate
    not MayAllocate solely for view construction

Arena.WithCapacity:
    ArenaCreate
    MayAllocate
    provider effects

Arena.Growable:
    ArenaCreate
    MayAllocate
    provider effects

Arena.New / Arena.Alloc:
    ArenaAllocate
    MayAllocate

Arena.Reset:
    ArenaReset
    AdvanceEpoch

Arena.Release:
    ArenaRelease
    EndDomain
    provider reclaim/end-borrow effects as applicable
```

---

# 67. Arena resource identity

High-level MLIR should expose an ArenaDomain-aware effect resource.

Conceptually:

```text
ArenaResource(ArenaDomainID)
```

This prevents invalid optimization across Reset/Release/allocation order.

---

# 68. Allocation operations are not CSE-safe

Two Arena allocations must not be common-subexpression-eliminated merely because
their operands appear equal.

They:

```text
consume different storage ranges
advance Arena control state
have ordered effects
produce distinct allocation bounds
```

---

# 69. Allocation may not move across Reset/Release

Optimization must not:

```text
hoist allocation across Reset
sink allocation across Release
move dependent use across Reset/Release
merge different epochs
relocate Arena allocations
```

---

# 70. Arena state plans

Add a read-only Sema/analysis fact.

Recommended:

```go
type ArenaBackingKind string
type ArenaGrowthKind string

type ResolvedArenaPlan struct {
    ArenaType             Type
    Domain                ArenaDomainID
    StateVersion          ArenaStateVersionID
    Epoch                 EpochDependency

    BackingKind           ArenaBackingKind
    GrowthKind            ArenaGrowthKind
    StorageFacts          StorageFacts
    ProviderPlan          ProviderPlan
    CapacityFacts         ArenaCapacityFacts

    OwnershipClass        CopyClassification
    DestructionPlan       DestructionPlanID
}
```

---

# 71. Arena operation plans

Recommended read-only plans:

```text
ResolvedArenaCreatePlan
ResolvedArenaAllocationPlan
ResolvedArenaResetPlan
ResolvedArenaReleasePlan
```

They contain all semantic decisions before builder execution.

---

# 72. Arena create plan

Records:

```text
constructor kind
fresh ArenaDomain identity
backing kind
growth kind
provider/backing source
initial epoch policy
capacity facts
fallibility
AllocationError type
storage classification
P17 ownership/destruction plan
```

---

# 73. Arena allocation plan

Records:

```text
ArenaDomain
state version before/after
element type
count
resolved layout/alignment
checked size plan
default plan
capacity proof/check
growth/provider plan
result kind New/Alloc
result reference facts
AllocationError
ordered effects
```

---

# 74. Arena reset plan

Records:

```text
ArenaDomain
current epoch
next-epoch/rekey strategy
dependency blockers already resolved
capacity reset behavior
state version before/after
ordered effect
```

The builder does not re-run liveness analysis.

---

# 75. Arena release plan

Records:

```text
ArenaDomain
backing kind
provider/reclamation plan
retained backing borrow
dependency blockers already resolved
P17 cleanup action
storage EndDomain plan
owner consumption
```

---

# 76. Read-only queries

Recommended:

```go
ResolvedArenaCreatePlanOf(ast.Node)
ResolvedArenaAllocationPlanOf(ast.Node)
ResolvedArenaResetPlanOf(ast.Node)
ResolvedArenaReleasePlanOf(ast.Node)
AllocationContextRequirementOf(FunctionID)
ActiveAllocationContextAt(ast.Node)
```

They must not mutate Analyzer state.

---

# 77. Allocation-context callable summary

Recommended:

```go
type AllocationContextRequirement string

const (
    AllocationContextNone     AllocationContextRequirement = "none"
    AllocationContextRequired AllocationContextRequirement = "required"
)
```

Keep `MayAllocate` as separate effect summary.

---

# 78. Context origin

Recommended:

```go
type AllocationContextOrigin string

const (
    AllocationContextExplicitArena AllocationContextOrigin = "explicit-arena"
    AllocationContextPropagated    AllocationContextOrigin = "propagated"
    AllocationContextCompilerLocal AllocationContextOrigin = "compiler-local"
    AllocationContextTarget        AllocationContextOrigin = "target"
)
```

No arbitrary lexical origin.

---

# 79. Basic Arena demand summary

P19 introduces compiler analysis data sufficient for bounded profiles.

Recommended expression kinds:

```text
constant
sum
maximum
constant-multiply
range-upper-bound
unknown
unbounded
```

This is compile-time analysis only.

---

# 80. Demand semantics

Within one Arena epoch:

```text
sequential allocations -> sum including alignment padding
exclusive branches -> maximum branch demand plus continuation
bounded repeated allocation -> checked multiplication/sum
Reset -> starts new demand window
zero-element allocation -> zero payload demand
unknown/open recursion -> unknown or unbounded
```

No general theorem prover is required.

---

# 81. Interprocedural demand boundary

P19 may consume existing closed direct-call summaries.

For:

```text
open indirect targets
unbounded recursion
unknown external contracts
spawned concurrent execution
```

the initial summary may conservatively become:

```text
unknown
or
unbounded
```

according to the target/profile rule.

Full concurrency-aware capacity analysis is deferred.

---

# 82. Strict profile validation

A strict/bare-metal profile may require bounded demand.

If the computed demand is:

```text
unknown
unbounded
or provably greater than fixed capacity
```

the compile must fail according to profile rules.

Hosted/permissive profiles may retain runtime/provider failure with
information/warning diagnostics.

---

# 83. Statically impossible allocation

If Sema/plan proves an allocation can never succeed under the selected fixed
Arena/profile:

```text
compile-time diagnostic
```

is preferred over generating a guaranteed failing `try` path.

The source Result type remains unchanged in metadata.

---

# 84. Semantic IR Arena type

Add:

```text
ArenaType
```

Source identity remains one builtin `Arena`.

Backing/growth/profile are value/plan facts, not separate source nominal types.

---

# 85. Semantic IR allocation-context type

Add compiler-internal:

```text
AllocationContextType
```

It is not source-visible.

It is not storable as ordinary user data.

---

# 86. Semantic IR Arena operations

Add:

```text
ArenaCreateBorrowedOp
ArenaCreateOwnedFixedOp
ArenaCreateGrowableOp

ArenaNewOp
ArenaAllocOp

ArenaResetOp
ArenaReleaseOp
ArenaDestroyOp

AllocationContextFromArenaOp
AllocationContextTargetOp
AllocationContextCompilerLocalOp
```

P18 storage-domain operations remain reusable.

---

# 87. Place-based Arena mutation

P19 uses P15 Places for mutating an Arena owner.

`ArenaNewOp`, `ArenaAllocOp`, and `ArenaResetOp` take:

```text
writable Place<Arena>
```

This models source receiver mutation without copying/moving the Arena owner.

The op updates semantic Arena state atomically.

---

# 88. Arena creation returns owned value

Arena constructors produce an owned Arena value.

The caller initializes an owned Place or transfers it according to P17.

---

# 89. `ArenaCreateBorrowedOp`

Input:

```text
mutable byte slice
```

Output:

```text
Arena owner
```

Infallible.

Consumes/transfers the mutable backing-slice borrow into the Arena backing
dependency for the Arena lifetime.

---

# 90. `ArenaCreateOwnedFixedOp`

Inputs:

```text
capacity
resolved provider/context facts
```

Outputs:

```text
Arena candidate
failed
AllocationError
```

On success:

```text
fresh domain live
owned backing established
```

On failure:

```text
candidate not consumed
no domain established
```

---

# 91. `ArenaCreateGrowableOp`

Same checked creation shape as owned fixed.

The resulting Arena's growth plan guarantees non-relocating existing
allocations.

Public source availability may remain gated.

---

# 92. `ArenaNewOp`

Input:

```text
writable Arena Place
```

Type parameter/fact:

```text
T
```

Outputs:

```text
ref mut T candidate
failed
AllocationError
```

On success the Arena state is updated.

On failure the Arena state is unchanged.

---

# 93. `ArenaAllocOp`

Inputs:

```text
writable Arena Place
count: uint
```

Type parameter/fact:

```text
T
```

Outputs:

```text
ref mut T[] candidate
failed
AllocationError
```

Success slice length equals exact requested count.

---

# 94. `ArenaResetOp`

Input:

```text
writable Arena Place
```

No source result.

Semantics:

```text
AdvanceEpoch
reset cursor(s)
retain backing
retain owner/domain
```

Dependency legality is pre-proven.

---

# 95. `ArenaReleaseOp`

Input:

```text
owned Arena value
```

No result.

Semantics:

```text
EndDomain
provider reclaim or backing-borrow end
consume Arena
```

P17 explicit Release moves/consumes the Arena from its source Place before this
op.

---

# 96. `ArenaDestroyOp`

P17 destruction-plan endpoint for an owned Arena.

Equivalent semantic terminal Release when still owned.

It exists to distinguish:

```text
explicit Release cause
implicit destruction cause
```

Physical implementation may later canonicalize them.

---

# 97. Nested Arena IR

`ArenaCreateBorrowedOp` may consume a mutable slice allocated from another Arena.

The child Arena operation records:

```text
child ArenaDomain
parent storage identity
parent ArenaDomain/epoch dependency
borrow bounds
```

No special nested-Arena source syntax.

---

# 98. Allocation context propagation in Semantic IR

A function summary records:

```text
RequiresAllocationContext
```

Functions requiring it receive one compiler context input in the high-level
function representation.

Direct call operations carry/forward the context explicitly in Semantic IR.

---

# 99. Context selection is explicit

For every implicit allocating operation, Semantic IR records one selected
context origin.

There is no operation with unresolved:

```text
"choose some arena"
```

semantics.

---

# 100. P18 dynamic-array integration

P18 allocation-capable owning-array operations that use ambient allocation
consume P19's resolved:

```text
AllocationContext
ProviderPlan
storage-origin/reclamation facts
```

P18 does not create its own competing context selection.

---

# 101. P16 slice integration

`Arena.Alloc[T]` result is the existing P16 mutable slice semantic type.

Its slice facts include:

```text
StorageOrigin = Arena
ArenaDomain identity
current epoch dependency
allocation bounds
mutable authority
borrow lifetime
```

---

# 102. P15 reference integration

`Arena.New[T]` result is the existing P15 `ref mut T`.

Its validity facts depend on:

```text
ArenaDomain
current Arena epoch
specific allocation bounds
```

---

# 103. P17 ownership integration

Arena:

```text
MoveOnly
non-trivially destructible
```

P17 cleanup tracks still-owned Arena values.

Explicit Release cancels the implicit Arena cleanup responsibility.

Arena destruction invokes terminal Release semantics.

---

# 104. P18 owning container Arena backing

An owning `T[]` may use Arena-backed storage according to P18.

The owning array:

```text
owns initialized element lifetimes
```

Arena:

```text
owns/controls raw storage domain
```

The array must end its Arena storage dependency before Reset/Release.

Arena does not become a destructor registry for the array elements.

---

# 105. Sec MLIR schema version 15

Compiler-generated high-level Sec MLIR uses:

```mlir
sec.dialect_version = 15 : i32
```

Schema versions 1 through 14 remain regression inputs.

Schema v15 adds:

```text
!sec.arena
!sec.alloc_context

sec.arena.create_borrowed
sec.arena.create_owned_fixed
sec.arena.create_growable
sec.arena.new
sec.arena.alloc
sec.arena.reset
sec.arena.release
sec.arena.destroy

sec.alloc_context.from_arena
sec.alloc_context.target
sec.alloc_context.compiler_local
```

P18 storage-domain ops are reused:

```text
sec.storage.establish_domain
sec.storage.advance_epoch
sec.storage.end_domain
sec.storage.reclaim
```

---

# 106. Why `!sec.arena` has no backing-kind type parameter

All source Arenas have the same builtin source type:

```text
Arena
```

Backing and growth policy are runtime/plan facts.

Keeping one high-level type makes:

```text
moves
function parameters/results
branch merges
generic storage
```

preserve source type identity without creating artificial subtype distinctions.

---

# 107. `!sec.alloc_context`

Compiler-internal capability/state.

It may appear:

```text
as hidden internal Sec function argument
as operand to compiler/core allocation operations
as result of target/compiler-local context selection
```

It may not appear in ordinary source data structures or source-visible function
types.

---

# 108. Hidden allocation-context function argument

A `func.func` requiring ambient context carries an argument with:

```text
type = !sec.alloc_context
sec.hidden_allocation_context = true
```

Exact argument position is compiler-defined but deterministic.

Recommended:

```text
after any existing compiler-hidden receiver/environment arguments
before ordinary source parameters
```

Keep one convention across the compiler.

---

# 109. Foreign ABI restriction

A `func.func` corresponding directly to ordinary foreign/export ABI must not
gain the hidden allocation-context argument unless:

```text
generated wrapper owns it
or
explicit ABI contract defines it
```

Verifier enforces this.

---

# 110. `sec.arena.create_borrowed`

Operand:

```text
!sec.slice_mut<byte>
```

Result:

```text
!sec.arena
```

Required facts:

```text
fresh domain ID
fixed growth
borrowed backing
parent storage/origin dependency
capacity
initial epoch policy
```

No allocation effect.

---

# 111. `sec.arena.create_owned_fixed`

Operand:

```text
capacity: uint
```

and abstract provider/context metadata.

Results:

```text
!sec.arena
i1 failed
AllocationError
```

Required fresh domain on success.

---

# 112. `sec.arena.create_growable`

Operand:

```text
initial capacity: uint
```

Results same checked shape.

Required growth plan:

```text
non-relocating existing allocations
```

---

# 113. `sec.arena.new`

Operand:

```text
!sec.place<!sec.arena,"rw">
```

Type parameter/fact T.

Results:

```text
!sec.ref_mut<T>
i1 failed
AllocationError
```

Operation is atomic and ordered on the Arena resource.

---

# 114. `sec.arena.alloc`

Operands:

```text
!sec.place<!sec.arena,"rw">
count: uint
```

Type parameter/fact T.

Results:

```text
!sec.slice_mut<T>
i1 failed
AllocationError
```

---

# 115. `sec.arena.reset`

Operand:

```text
!sec.place<!sec.arena,"rw">
```

No result.

Required:

```text
ArenaDomain
state before/after
epoch transition strategy
```

Emits/contains `AdvanceEpoch` semantic effect.

---

# 116. `sec.arena.release`

Operand:

```text
!sec.arena
```

Consumes owner.

No result.

Required:

```text
ArenaDomain
backing kind
provider/reclamation plan
release cause
```

---

# 117. `sec.arena.destroy`

Operand:

```text
!sec.arena
```

Consumes owner.

No result.

Cause:

```text
implicit destruction
```

Semantics terminal Release.

---

# 118. `sec.alloc_context.from_arena`

Input:

```text
resolved mutable Arena authority
```

Result:

```text
!sec.alloc_context
```

It does not change ambient context globally.

It is valid only for an operation/call contract explicitly selecting that Arena.

---

# 119. `sec.alloc_context.target`

Produces an abstract target-provided context at a supported Sec entry root.

No universal provider is assumed.

---

# 120. `sec.alloc_context.compiler_local`

Produces a compiler-managed context only after proof:

```text
lifetime bounded
non-owning escape absent
backing strategy concrete
failure semantics preserved
profile allows
```

Its physical Arena may later be scalarized/eliminated.

---

# 121. Arena state-version metadata

Mutating operations carry/reference:

```text
sec.arena_state_before
sec.arena_state_after
```

or equivalent typed analysis metadata.

State version changes do not imply epoch changes.

---

# 122. ArenaDomain metadata

Arena ops and returned references/slices carry/reference:

```text
sec.arena_domain
sec.validity_epoch_dependency
sec.storage_identity
```

using canonical typed attrs/IDs.

Do not encode source variable names as identity.

---

# 123. Arena MLIR effects

Arena ops integrate with:

```text
MemoryEffectOpInterface where meaningful
Sec-specific ordered Arena effect interface/resource
```

Standard `Allocate`/`Free` alone are insufficient.

Reset is not ordinary Free.

---

# 124. Arena verifier

Register:

```bash
--sec-verify-arenas
```

It validates local and cross-op Arena invariants.

---

# 125. Allocation-context verifier

Register:

```bash
--sec-verify-allocation-contexts
```

It validates context requirement/propagation/foreign boundaries.

---

# 126. Arena verifier requirements

Check:

```text
Arena type is MoveOnly
fresh domain on creation
same domain preserved by move
create-borrowed input mutable byte slice
borrowed backing retained until Release
safe T requirements for New/Alloc
count/result shape
state-version monotonic semantic flow
ordinary allocation does not change epoch
Reset advances epoch
Release ends domain
no use after Release
no double Release
implicit destroy and explicit Release do not both consume owner
nested child dependency
allocation failure atomicity
```

---

# 127. Dependency verifier extension

Extend P15/P16/P18 verifiers so:

```text
Arena ref/slice dependency uses current domain epoch
Reset rejects/does not appear with live strong dependency
Release rejects/does not appear with live strong dependency
nested Arena blocks parent invalidation
```

The verifier checks emitted facts; Sema remains the source liveness authority.

---

# 128. Allocation-context verifier requirements

Check:

```text
function requiring context has one hidden context
function not requiring context does not require one
direct call forwards context when required
explicit Arena selection does not rebind ambient context
lexical Arena is never guessed
no-context operation does not reach MLIR from valid Sema
spawn/thread boundary does not inherit parent context by default
foreign ABI does not receive hidden context without wrapper/contract
```

---

# 129. Arena effect verifier

ArenaResource ordering must prevent:

```text
CSE of allocation
allocation crossing Reset
use crossing Release
Reset/Release reordering
epoch merge
invalid hoisting from loop
```

---

# 130. Demand-summary representation

Module/callable metadata may store:

```text
sec.arena_demand_summary
sec.requires_allocation_context
sec.may_allocate
```

The demand summary is compile-time metadata, not runtime values.

---

# 131. Strict profile verifier

A CompilationPlan may require:

```text
bounded Arena demand
known provider
no ambient context
no growth
no runtime epoch metadata
```

P19 validates only the rules declared by the active profile.

Source Arena semantics remain stable.

---

# 132. Hosted profile

Typically permits:

```text
FromBuffer
WithCapacity
growable Arena strategy
compiler-managed ambient allocation context
```

Provider failure remains represented unless proven impossible.

---

# 133. Embedded fixed profile

Typically prefers:

```text
borrowed fixed Arena
static backing
caller-provided backing
target-provided pools
```

Owned/growable providers remain target-dependent.

---

# 134. Bare-metal bounded profile

May require:

```text
all relevant capacity statically bounded
no operating-system provider
no hidden growth
no mandatory runtime epoch metadata
no ambient allocation context without explicit backing
```

A fully static Arena may later be descriptor-eliminated.

---

# 135. No-allocation profile

P19 preserves:

```text
Arena.FromBuffer may remain legal
provider-backed Arena creation unavailable unless profile says otherwise
ambient allocating operations invalid
no hidden fallback
```

`Arena.New/Alloc` remain allocation effects and may be rejected by the profile
or `@noAlloc` guarantee even with already acquired backing.

---

# 136. Effect-analysis integration

P19 requires read-only compiler facts for:

```text
ArenaCreate
ArenaAllocate
ArenaReset
ArenaRelease
MayAllocate
RequiresAllocationContext
provider effects
storage-domain transitions
```

P19 does not replace the canonical effect analysis.

---

# 137. Call-graph integration

Callable/call-site summaries retain:

```text
allocation-context requirement
Arena effects
explicit Arena identity where known
Arena demand summary
provider requirement
open-callable status
```

P19 handles synchronous propagation.

Task/thread execution relationships remain metadata hooks for later concurrency
lowering.

---

# 138. Separate compilation metadata

Public/module metadata must preserve at least:

```text
RequiresAllocationContext
declared/inferred allocation effects
Arena demand summary
explicit Arena parameter summaries
provider requirements
open callable bounds
trust provenance
```

Changing these may require dependent recompilation.

---

# 139. P17 cleanup integration

Arena local initialization registers `DestroyArena`.

Explicit Release:

```text
consumes Arena
cancels automatic Arena cleanup
```

Normal function/scope cleanup:

```text
destroys still-owned Arena
performs terminal Release
```

Defer ordering remains unified P17 cleanup order.

---

# 140. Defer interaction

A defer that uses Arena-backed storage blocks an earlier Release/Reset cleanup
ordering.

Example conceptual registration order must preserve:

```text
dependent use executes before Release
```

P19 consumes P17 capture-place and cleanup ordering facts.

---

# 141. Task/thread boundary scope

P19 does not implement full spawn/await/join/thread lowering.

It must still preserve canonical facts:

```text
spawned context does not inherit mutable parent Arena context
Arena dependency may be captured/transferred
Reset/Release blockers may include task/thread dependency
completion metadata can later discharge dependency
```

The concurrency package will lower those execution relationships.

---

# 142. FFI retention boundary

P19 does not implement complete FFI retention lowering.

Arena-backed foreign retention facts remain attached to the existing FFI
contract/provenance model.

Unknown retention is conservative.

`RawPtr[T]` alone does not keep Arena alive.

---

# 143. No individual Arena allocation free

Sec 0.1 has no individual `Free` operation for Arena allocations.

Ending a returned reference/slice:

```text
does not reclaim Arena capacity
```

Capacity is reused only through Reset/Release or another future defined Arena
operation.

---

# 144. No uninitialized safe Arena allocation

P19 does not expose uninitialized Arena slots as:

```text
ref mut T
ref mut T[]
```

General safe uninitialized typed Arena allocation remains outside Sec 0.1
initial Arena API.

---

# 145. No arbitrary non-trivial `T` in Arena.New/Alloc

Even after P17:

```text
files
locks
owning containers
custom-destructor types
other non-trivially-destructible T
```

remain invalid direct `Arena.New/Alloc` element types.

Owning containers may internally use Arena backing under their own destruction
protocol.

---

# 146. No universal physical Arena representation

P19 does not select:

```text
base/capacity/cursor struct
segment-list layout
provider vtable
epoch field layout
stack Arena layout
static Arena layout
virtual-reserve layout
```

These are later CompilationPlan physical choices.

---

# 147. No universal Arena ABI

`Arena` and `!sec.alloc_context` are not ordinary FFI-stable ABI values.

Foreign wrappers require explicit contracts.

---

# 148. No mandatory runtime

P19 does not require:

```text
heap
GC
reference counting
global Arena registry
global allocator singleton
runtime borrow checker
universal epoch table
universal unwinder
```

A bounded Arena may lower entirely to static/stack offsets or be eliminated.

---

# 149. Required source/frontend tests

```text
Arena builtin type
lowercase arena identifier allowed
Arena copy rejected
Arena move accepted
FromBuffer valid
WithCapacity valid
Growable source only when enabled
New valid
Alloc valid
zero-element Alloc
Reset valid after last use
Reset invalid with live ref/slice
Release consumes
use after Release invalid
double Release invalid
return Arena owner valid
return local Arena ref invalid
nested Arena parent reset invalid
```

---

# 150. Required type-restriction tests

```text
New/Alloc sized T
unsized rejected
missing default rejected
non-trivially-destructible rejected
wide active int128/uint256 accepted
decimal128 accepted when layout/default is valid
alignment-invalid provider case
checked count*size overflow
```

---

# 151. Required allocation atomicity tests

```text
fixed success
fixed capacity failure
growable segment success
growable provider failure
failure leaves Arena state version unchanged
failure leaves epoch unchanged
failure leaves old allocations valid
no partial ref/slice published
```

---

# 152. Required Reset tests

```text
fixed Reset
borrowed Reset
growable Reset
epoch advance
state-version advance
capacity retained
cursor reset
bytes not semantically zeroed
old ref cannot cross
old slice cannot cross
nested child blocks
defer dependency blocks
epoch metadata elimination when proven
```

---

# 153. Required Release/destruction tests

```text
owned Release reclaims provider backing
borrowed Release ends borrow only
static Release no deallocation
target-provided Release follows provider plan
implicit destroy performs Release
explicit Release cancels implicit cleanup
moved Arena source not released
return Arena transfers cleanup
```

---

# 154. Required allocation-context tests

```text
MayAllocate without RequiresAllocationContext via explicit Arena
ambient required callable
sync propagation A->B->C
function not requiring context gets none
lexical Arena not guessed
explicit Arena selection
compiler-local context only with proof
target-provided context
missing context compile error
spawn boundary no inheritance
foreign export hidden-context rejection
```

---

# 155. Required demand-summary tests

```text
constant sequential demand
alignment padding
branch maximum
continuation after branch
bounded loop multiplication
range upper bound
Reset starts new peak window
zero allocation
unknown loop
unbounded recursion
open callable unknown
fixed capacity impossible
strict profile bounded success
hosted unknown accepted with diagnostic policy
```

---

# 156. Required Semantic IR tests

```text
ArenaCreateBorrowedOp
ArenaCreateOwnedFixedOp
ArenaCreateGrowableOp
ArenaNewOp
ArenaAllocOp
ArenaResetOp
ArenaReleaseOp
ArenaDestroyOp

AllocationContextFromArenaOp
AllocationContextTargetOp
AllocationContextCompilerLocalOp

ArenaDomain identity
state version versus epoch
nested domain dependency
AllocationError flow
atomic failure
ordered effect sites
```

---

# 157. Required Sec MLIR dialect tests

Schema v15:

```text
!sec.arena round-trip
!sec.alloc_context round-trip
all arena ops parse/print/verify
all context ops parse/print/verify
hidden function context arg
ArenaDomain attrs
state-version attrs
effect interface/resource
AllocationError result types
schema-v14 regression
```

---

# 158. Required Arena verifier negative tests

```text
copy Arena
create borrowed from shared slice
create borrowed from wrong element type
New non-trivial T
Alloc wrong count type
Alloc result wrong slice type
ordinary allocation changes epoch
Reset without epoch transition
Release without EndDomain
use after Release
double Release
same domain reused after terminal Release
child dependency omitted
```

---

# 159. Required allocation-context verifier negative tests

```text
required function missing context
unneeded hidden context required as source param
direct call fails to forward
wrong context forwarded
lexical Arena silently selected
spawn inherits parent mutable context
ordinary foreign ABI has hidden context
compiler-local context with escaping ref
no-context allocation reaches IR
```

---

# 160. Required integration tests

P15:

```text
Arena.New ref mut dependency
Reset/release invalidation
```

P16:

```text
Arena.Alloc mutable slice
slice range/index
```

P17:

```text
Arena move
Arena implicit destruction
explicit Release cleanup cancellation
defer ordering
```

P18:

```text
dynamic owning array uses resolved Arena allocation context
Arena-backed dynamic array destroys elements before Arena reset/release
no competing storage model
```

---

# 161. Explicitly deferred

P19 does not implement:

```text
physical Arena descriptor
physical provider ABI
malloc/free selection
general uninitialized Arena allocation
individual Arena allocation Free
full task/thread spawn/await/join lowering
concurrent Arena
stable/weak handles
full FFI retention lowering
full physical panic cleanup/unwinder
public capacity properties
public arbitrary Arena trimming
general collection types
LLVM Arena lowering
```

---

# 162. Architecture rules

Non-negotiable:

```text
Arena is one move-only builtin source type.

ArenaDomain identity is independent of physical backing address.

Arena state version is not the same as validity epoch.

Ordinary allocation changes state, not epoch.

Reset preserves ArenaDomain and advances epoch.

Release consumes Arena and ends ArenaDomain.

FromBuffer is infallible borrowed fixed Arena construction.

WithCapacity is fallible owned fixed Arena construction.

Growable Arena may add stable backing only; it may never relocate existing live
Arena allocations.

Arena.New/Alloc publish only fully initialized T values.

Arena.New/Alloc initially require trivially destructible T even though P17 can
represent general destruction.

Allocation failure uses AllocationError and is atomic.

Zero-element Alloc succeeds without capacity/growth/epoch change.

Repeated allocation does not invalidate prior allocations.

Arena allocations are not individually freed.

Arena object ownership and physical backing ownership are distinct.

Nested Arena has its own domain and depends on parent storage/epoch.

Allocation context is compiler-visible, not source-global.

MayAllocate and RequiresAllocationContext are distinct.

Explicit Arena selection beats ambient context.

Lexical Arena values are never guessed as ambient context.

Synchronous internal calls propagate context only when required.

Ordinary foreign ABI never receives hidden context without explicit contract.

No-allocation profiles never trigger hidden heap fallback.

Arena effects are ordered and ArenaDomain-aware.

CSE/reordering may not erase Arena allocation semantics.

P17 cleanup owns implicit Arena destruction.

P18 storage transitions are reused; Arena does not create a competing epoch model.

No mandatory runtime is introduced.

No physical Arena representation or allocator ABI is selected.

No LLVM dialect is generated by P19.
```

---

# 163. Acceptance criteria

Package 19 is complete only when:

```text
[ ] baseline documents repo 152c772 + local P13-P18 or newer equivalent
[ ] previous package regressions remain green
[ ] Arena/allocation synchronization applied
[ ] Semantic IR Arena amendment applied
[ ] schema-v15 dialect rulebook installed
[ ] lowering-v15 rulebook installed
[ ] Arena builtin normalized
[ ] lowercase arena keyword removed if still reserved
[ ] Arena MoveOnly classification integrated with P17
[ ] ArenaDomainID implemented
[ ] ArenaStateVersionID implemented
[ ] backing/growth facts implemented
[ ] FromBuffer implemented
[ ] WithCapacity implemented
[ ] growable semantic model implemented
[ ] safe New[T] implemented
[ ] safe Alloc[T] implemented
[ ] safe type restrictions enforced
[ ] AllocationError mapping implemented
[ ] zero-element allocation implemented
[ ] atomic failure semantics implemented
[ ] ordinary allocation preserves epoch
[ ] Reset implements AdvanceEpoch
[ ] Reset dependency validation integrated
[ ] Release implements EndDomain
[ ] release-by-backing-kind implemented
[ ] implicit Arena destruction integrated with P17
[ ] nested Arena dependency implemented
[ ] allocation-context requirement summary implemented
[ ] MayAllocate kept distinct
[ ] context selection order implemented
[ ] hidden !sec.alloc_context internal propagation implemented
[ ] lexical Arena never auto-selected
[ ] foreign ABI hidden-context guard implemented
[ ] noalloc profile behavior implemented
[ ] ordered Arena effects represented
[ ] basic Arena demand summary implemented
[ ] strict profile capacity checks implemented
[ ] ArenaResource/effect interface implemented
[ ] !sec.arena implemented
[ ] !sec.alloc_context implemented
[ ] schema-v15 Arena/context ops implemented
[ ] --sec-verify-arenas registered
[ ] --sec-verify-allocation-contexts registered
[ ] P15/P16 dependency verifiers extended
[ ] P18 allocation-context integration implemented
[ ] no public uninitialized allocation added
[ ] no arbitrary non-trivial Arena.Alloc added
[ ] no physical Arena descriptor selected
[ ] no allocator ABI selected
[ ] no mandatory runtime
[ ] check-sec-mlir passes
[ ] go test ./... passes
[ ] legacy paths remain operational
```

---

# 164. Required implementation report

Codex must report:

```text
1. repository HEAD implemented against
2. local/merged P13-P18 status
3. previous package status
4. Arena/allocation normative synchronization
5. files added
6. files modified
7. Arena builtin normalization
8. lowercase arena keyword status
9. MoveOnly integration
10. ArenaDomain implementation
11. Arena state-version implementation
12. backing/growth/provider facts
13. FromBuffer implementation
14. WithCapacity implementation
15. growable semantic model
16. New[T] validation/lowering
17. Alloc[T] validation/lowering
18. AllocationError mapping
19. zero-element allocation
20. allocation atomicity
21. Reset dependency/epoch implementation
22. Release/backing-kind implementation
23. implicit destruction integration
24. nested Arena dependency
25. allocation-context summary
26. context selection/propagation
27. hidden Sec MLIR context parameter
28. foreign ABI context guard
29. noalloc behavior
30. ordered Arena effect implementation
31. ArenaResource interface
32. capacity-demand summary
33. strict profile validation
34. schema-v15 types/ops
35. Arena verifier
36. allocation-context verifier
37. P15/P16 verifier extensions
38. P17 cleanup integration
39. P18 allocation-context integration
40. wide-type Arena tests
41. borrowed/owned/growable tests
42. Reset/Release tests
43. demand/profile tests
44. unsupported concurrency/FFI/uninitialized tests
45. CMake commands
46. exact LLVM/MLIR version
47. check-sec-mlir result
48. go test ./... result
49. end-to-end source -> schema-v15 results
50. deviations
51. recommendations for Package 20
```

---

# 165. Package 20 boundary

Recommended Package 20:

```text
Floating-Point Semantic Operations and Arith Lowering
```

Reason:

The new pipeline now has scalar integer semantics, allocation/storage, ownership,
aggregates and reference infrastructure.

The next independent core gap is complete floating-point operation semantics.

Recommended P20 scope:

```text
float / float32 / float64 high-level scalar ops
literal exact-to-target conversion policy
IEEE-compatible arithmetic
NaN/infinity/signed-zero behavior
ordered comparisons
float remainder semantics
classification intrinsics
explicit integer/float conversions
constant folding parity with runtime
Sec MLIR float ops or canonical Arith mapping
target float capability checks
no accidental fast-math
no LLVM yet
```

P21 should then handle:

```text
decimal
decimal128
checked decimal arithmetic
scale/precision semantics
decimal conversions
physical decimal lowering boundary
```
