# Semantic IR

- Status: Normative
- Created: 2026-09-03
- Last updated: 2026-09-03
- Document revision: 2.0
- Sec language version: 0.1
- Canonical path: `rules/compiler/semantic_ir.md`
- Replaces: `rules/compiler/semantic_ir.txt`
- Repository baseline reviewed: `998d8d1` (latest publicly verifiable `main` while this revision was prepared; the current Iterator[T] design is incorporated as a later project decision)

---

## § 1 Purpose and authority

§ 1(1) Sec Semantic IR is the canonical typed representation of a semantically valid Sec program after frontend semantic analysis and before Sec MLIR lowering.

§ 1(2) Semantic IR represents what the validated program means.

§ 1(3) The AST represents how the program was written.

§ 1(4) Sec MLIR represents how validated meaning is progressively lowered.

§ 1(5) LLVM IR or another target backend represents target implementation details.

§ 1(6) Semantic IR must not redefine Sec language semantics.

§ 1(7) Semantic IR must not depend on source syntax that has already been semantically resolved.

§ 1(8) Semantic IR generation must fail when unresolved semantic errors remain.

§ 1(9) Semantic IR is the final compiler-owned authority for validated Sec semantics before lowering begins.

§ 1(10) Specialized language rulebooks remain authoritative for their source semantics; this rulebook defines which semantic facts and operations must survive into the canonical IR.

---

## § 2 Position in the compiler

§ 2(1) Semantic IR is produced only after all analyses required to validate the represented operation have completed.

§ 2(2) Depending on the construct, required frontend work may include:

```text
parsing
name resolution
type resolution
generic/interface conformance resolution
compile-time evaluation
control-flow validation
ownership analysis
copy/move classification
borrow analysis
lifetime analysis
escape analysis
destruction/cleanup planning
error/try resolution
effect analysis
allocation-context resolution
storage/reference analysis
transferability/concurrency analysis
target-independent platform validation
target-plan resolution where semantically required
```

§ 2(3) A later pass may refine representation facts without rerunning source-level semantic resolution.

§ 2(4) Lowering must consume Semantic IR facts rather than rediscover Sec semantics from AST naming patterns, LLVM types, or backend behavior.

---

## § 3 Primary goals

§ 3(1) Semantic IR must:

```text
represent Sec semantics independently of source spelling
use canonical semantic identities
make ownership-changing operations explicit
make availability-changing operations explicit
make destruction and cleanup explicit
make control flow explicit
make failure and panic edges explicit where relevant
preserve reference/storage validity facts
preserve effect and trust facts
preserve target/platform semantic requirements
preserve source provenance
support deterministic verification
provide stable input to Sec MLIR lowering
support compiler/LSP diagnostic inspection
remain independent of LLVM representation
```

§ 3(2) Semantic IR is not an untyped instruction stream.

§ 3(3) Semantic IR is not a direct copy of the AST.

§ 3(4) Semantic IR is not LLVM IR.

§ 3(5) Semantic IR is not an optimization-specific machine representation.

---

## § 4 Required module invariants

§ 4(1) Every Semantic IR module must satisfy at least:

```text
every symbol reference is resolved
every represented value has a canonical type
every direct call has a resolved target identity
every indirect call has a resolved callable contract
every method/property/interface operation is semantically resolved
every generic use has a concrete or canonical specialization strategy
every ownership transfer is explicit
every semantic copy is explicit until proven erasable
every semantic move is explicit
every Place availability transition is explicit or canonically recoverable
every required destruction point is explicit or represented by a cleanup plan
every reference has a canonical origin/storage dependency
every borrow was validated
every block target exists
every block terminates explicitly
every branch argument matches the destination semantic state
every return satisfies the resolved callable contract
every unsafe operation has validated unsafe context/provenance
every FFI call has a resolved ABI/linkage contract
every runtime check has resolved failure semantics
every panic-producing operation has a stable panic reason/contract where required
every target-dependent semantic operation has sufficient CompilationPlan facts
every readable defaulted value is initialized
no unresolved source-level overload/member/iterator discovery remains
```

§ 4(2) Violation of a Semantic IR invariant after successful frontend validation is an internal compiler error.

---

## § 5 Module representation

§ 5(1) A Semantic IR module may contain:

```text
module identity
source-file identities
imports/exports
symbol table
type declarations
static/global declarations
function declarations/definitions
extern declarations
interface/conformance metadata
generic templates where retained
concrete specializations
compiler-known semantic identities
target/profile restrictions
build-condition results
source-location tables
effect summaries
separate-compilation summaries
```

§ 5(2) A Semantic IR module must not contain LLVM type objects or LLVM instructions.

§ 5(3) Imported semantic facts must be validated/version-compatible before being used as proof.

---

## § 6 Symbol identity

§ 6(1) Every declaration receives a unique semantic symbol identity.

§ 6(2) Symbol identity distinguishes declarations with identical source names in different scopes/modules.

§ 6(3) Semantic IR references use symbol identities rather than repeated name lookup.

§ 6(4) Display names remain available for diagnostics/debugging.

§ 6(5) Stable cross-compilation identities may additionally exist for:

```text
exported symbols
public types
concrete generic specializations
compiler-known declarations
linker-visible symbols
incremental compilation artifacts
generated platform artifacts
```

§ 6(6) Cross-compilation identity must not depend on process-local pointer values or nondeterministic map order.

---

## § 7 Type identity

§ 7(1) Semantic IR uses canonical Sec type identities.

§ 7(2) Canonical types include all Sec 0.1 scalar, aggregate, reference, callable, interface, collection, shaped, error, unit, register, platform, and compiler-known types.

§ 7(3) Type identity remains semantic even when multiple types later share one machine representation.

§ 7(4) Named/distinct types must not be silently reduced to their carrier/base type.

§ 7(5) Physical representation must not replace semantic identity before all semantic consumers are complete.

---

## § 8 Type facts

§ 8(1) A canonical Semantic IR type record or equivalent service preserves where relevant:

```text
TypeID
nominal identity
base/carrier relationship
signedness
semantic numeric width
target-sized status
mutability semantics
copy classification
move-only status
destruction classification
defaultability
field/variant definitions
generic arguments
interface/conformance facts
unit semantics
register bit layout
reference category
collection/shaped metadata
ABI/layout requirements
unsafe-callable status
callable receiver/parameter ownership modes
```

§ 8(2) Plan-resolved physical layout may be referenced rather than embedded directly.

---

## § 9 Values

§ 9(1) Every Semantic IR value has:

```text
ValueID
canonical type
defining operation or parameter origin
ownership/value classification where relevant
source provenance where applicable
```

§ 9(2) Values may represent:

```text
function/block parameters
constants
owned values
borrowed references
raw pointers
aggregate values
Result/Option values
closure values
task/thread/process handles
Arena state values
collection/shaped descriptors
control-flow merge values
compiler-known semantic tokens
```

§ 9(3) Semantic IR should use SSA-style values where practical.

§ 9(4) Source mutability does not require mutable SSA values.

---

## § 10 Places and semantic locations

§ 10(1) Semantic IR must preserve canonical Place/location identity wherever ownership, borrowing, mutation, destruction, races, storage validity, or diagnostics depend on it.

§ 10(2) A Place identity consists conceptually of:

```text
root semantic identity
projection path
storage identity where materialized
```

§ 10(3) Projections may include:

```text
field
constant index
dynamic index
range/view
variant payload
dereference
compiler-known projection
```

§ 10(4) Place identity must not be reconstructed from source spelling after Sema.

§ 10(5) Place relationship facts such as Same, Disjoint, Contains, ContainedBy, MayOverlap, or Unknown may be retained/referenced where required.

---

## § 11 Constants

§ 11(1) Semantic IR constants use Sec semantics, not host-language arithmetic.

§ 11(2) Constants may include:

```text
integer
decimal
floating-point
bool
char/rune
string
enum
aggregate
unit
address
compiler-known identity
target-sized constant
```

§ 11(3) Integer constants retain arbitrary precision as needed until checked target/value conversion.

§ 11(4) Target-sized constants use the selected target, never compiler-host width.

§ 11(5) `null` is represented only where the owning raw/FFI semantics permit it.

---

## § 12 Compile-time values

§ 12(1) Compile-time-only values do not become runtime operations unless they intentionally materialize runtime data.

§ 12(2) Semantic IR may retain compile-time metadata for:

```text
specialization identity
generic value substitution
layout/target selection
units
registers
attributes
diagnostics
debug information
constant proof
```

§ 12(3) Semantic IR must not rerun general compile-time evaluation.

§ 12(4) Compile-time declarations with no runtime representation may be omitted from runtime lowering.

---

## § 13 Struct definitions

§ 13(1) Struct representation uses canonical semantic field identities.

§ 13(2) `StructFieldID` is a declaration-order semantic identity local to one nominal struct definition.

§ 13(3) `StructFieldID` is not a physical byte offset.

§ 13(4) A struct definition retains where relevant:

```text
TypeID
SymbolID
concrete generic arguments
declaration-order stored fields
field tags/annotations
copy/destruction/defaultability facts
optional ResolvedLayout identity
source provenance
synthetic-origin metadata where compiler-generated
```

§ 13(5) Properties and non-storage members are excluded from stored-field layout.

---

## § 14 Struct construction plans

§ 14(1) Struct literals are built from immutable Sema-resolved plans.

§ 14(2) A plan preserves source evaluation order independently of declaration-order final field materialization.

§ 14(3) Every stored field has exactly one final semantic origin such as:

```text
explicit source field
spread field
canonical default
```

§ 14(4) Transfer action per field must already be resolved.

§ 14(5) Semantic IR must not depend on synthesized legacy AST default nodes.

§ 14(6) Construction exposes no uninitialized semantic field.

---

## § 15 Struct operations

§ 15(1) Semantic IR supports distinctions equivalent to:

```text
StructConstruct
StructSpreadFields
StructExtractField
StructBorrowField
StructReplaceField
StructMoveField
StructCopyField
StructDestroyRemaining
```

§ 15(2) Exact operation names are implementation details.

§ 15(3) Field extraction/replace uses semantic field identity rather than physical offset.

§ 15(4) Functional aggregate replacement must remain distinct from in-place storage replacement when ownership/destruction behavior differs.

---

## § 16 Arrays, slices, and ranges

§ 16(1) Arrays retain element type and compile-time length.

§ 16(2) Safe slices/views retain:

```text
element type
storage/reference origin
length/extent
mutability authority
bounds
lifetime/storage dependency
```

§ 16(3) Index operations distinguish checked, proven-safe, and explicitly unsafe/unchecked forms.

§ 16(4) Range values preserve inclusive/exclusive bound semantics until canonical iteration/index lowering has made them explicit.

§ 16(5) Slice/view creation does not imply ownership of backing storage.

---

## § 17 Strings

§ 17(1) Strings remain semantically distinct from byte arrays and foreign C strings.

§ 17(2) String values retain encoding/semantic identity, ownership/materialization facts, and required storage/reference dependencies.

§ 17(3) String literals may reference immutable static backing.

§ 17(4) String materialization/allocation effects remain explicit.

§ 17(5) Conversion to foreign strings remains an explicit FFI operation.

---

## § 18 Collections

§ 18(1) Semantic IR must represent compiler-known collection semantics explicitly until safely lowered.

§ 18(2) Relevant collection categories include at least:

```text
list
map
set
vector
```

and other canonical Sec collection categories.

§ 18(3) Collection values retain where relevant:

```text
collection kind
element/key/value type
ownership classification
backing-storage identity
length/capacity semantics
allocation context/domain
mutation/invalidation contract
iterator/view dependency
effects
```

§ 18(4) A compiler-known collection operation must not be reconstructed from a generic method name after Sema.

§ 18(5) Structural mutation that can invalidate references/iterators must remain distinguishable from element mutation.

---

## § 19 Shaped values

§ 19(1) Semantic IR must represent shaped types without flattening their semantic structure prematurely.

§ 19(2) Relevant categories include:

```text
matrix
tensor
tensor_view
other canonical fixed/dynamic shaped forms
```

§ 19(3) Shaped values retain where relevant:

```text
element type
rank
shape
strides
layout
memory space
ownership/view relationship
bounds
broadcast status
allocation/materialization effects
```

§ 19(4) Scalar indexing that changes rank and range indexing that preserves dimensions must remain semantically distinguishable.

§ 19(5) Borrowed shaped views remain non-owning and retain backing-storage/reference dependency.

§ 19(6) Shape compatibility checks and their recoverable/panic semantics remain explicit until proven/eliminated.

---

## § 20 Units

§ 20(1) Unit semantics remain attached until every unit-sensitive operation has been validated and made explicit.

§ 20(2) Semantic IR preserves where relevant:

```text
carrier
unit identity
structural factors
dimension
category
kind/system
transform
scale
offset/origin
log parameters
point/difference role
conversion plan
exactness
```

§ 20(3) Named and structurally equivalent units may remain semantically distinct.

§ 20(4) Unit erasure is permitted only after all unit-dependent semantics are explicit/proven.

---

## § 21 Registers

§ 21(1) Register types retain their semantic bit layout and access contracts.

§ 21(2) Semantic IR preserves:

```text
register width
field identities/bit ranges
reserved ranges
field units
permissions
special read/write semantics
volatile/hardware contract
address/resource identity where bound
```

§ 21(3) Register field access is represented as semantic register/hardware operations, not inferred later from integer masks.

---

## § 22 Enums

§ 22(1) Enum values retain enum identity and selected member/value semantics.

§ 22(2) String-backed enums preserve their string representation contract rather than being forced to integer representation.

§ 22(3) Bit-backed/open hardware enum semantics remain distinct from closed ordinary enum semantics.

§ 22(4) Enum-to-carrier and carrier-to-enum conversions are explicit.

§ 22(5) Checked conversion validity is preserved until proven/lowered.

---

## § 23 Tagged unions

§ 23(1) Tagged unions retain active-variant semantics independent of later physical layout.

§ 23(2) Semantic operations include construct/test/extract/borrow/move/copy/destroy-active-payload distinctions.

§ 23(3) Safe payload access occurs only after validated variant selection.

§ 23(4) Reachable compiler-known empty state for mutable unions is represented explicitly when canonical union rules permit it.

§ 23(5) Foreign untagged-union access remains an explicit unsafe/FFI operation.

---

## § 24 Result and Option

§ 24(1) `Result[T,E]` and `Option[T]` retain their semantic alternatives until representation lowering.

§ 24(2) Semantic IR supports explicit construct/test/project operations.

§ 24(3) Consuming `.Ok()`/`.Err()` projections preserve ownership transfer.

§ 24(4) Borrowed projections preserve reference origin/borrow semantics.

§ 24(5) `Option` must not be assumed to be a nullable pointer unless a verified representation pass selects that optimization.

§ 24(6) Result representation must not be replaced by an arbitrary integer/error-code convention before ABI/target lowering.

---

## § 25 Error-root identity

§ 25(1) The compiler-known `error` root remains a semantic assignability relation, not an object-oriented base-class representation.

§ 25(2) Semantic IR preserves:

```text
specific error identity
payload identity
widened/open error-root identity where applicable
precise versus open Result channel
```

§ 25(3) Widening must not lose specific payload/identity facts required by later matching, diagnostics, or lowering.

§ 25(4) Physical error representation is deferred to verified ABI/layout lowering.

---

## § 26 Properties

§ 26(1) Property access remains semantically distinct from field access.

§ 26(2) Semantic IR distinguishes:

```text
property read
infallible property write
fallible property write
compound property update
```

or an equivalent expanded form preserving one receiver evaluation.

§ 26(3) Property operations preserve receiver evaluation order, ownership/borrow effects, setter error edges, cleanup, and defer behavior.

§ 26(4) Lower stages must not infer property semantics from field loads/stores.

---

## § 27 Spread and variadic packs

§ 27(1) Spread semantics remain explicit until normalization preserves:

```text
source evaluated exactly once
destination context
source expansion order
copy/borrow/move action per expansion
struct override order
default application order
no hidden allocation
```

§ 27(2) Native variadic packs remain distinct from arrays/slices/foreign varargs until legal lowering.

§ 27(3) Native variadic packs are call-lifetime, structurally read-only, non-escaping, and do not permit individual move-out unless a canonical rule says otherwise.

§ 27(4) Lowering must not introduce semantic heap allocation merely to materialize a pack.

---

## § 28 Static storage and associated values

§ 28(1) Semantic IR preserves static/value-associated semantics independently of instance representation.

§ 28(2) Relevant semantic entities/operations include:

```text
StaticStorage
StaticLoad
StaticStore
StaticInitialize
StaticDestroy
TypeAssociatedImmutable
InstanceAssociatedImmutable
StaticMethod
StaticProperty
```

§ 28(3) `let` versus `static let` lookup semantics in `impl` must remain distinguishable where lookup/visibility matters.

§ 28(4) Static members never become instance fields.

§ 28(5) Static initialization semantics must not imply hidden runtime startup/lazy functions unless the owning rule explicitly requires them.

---

## § 29 Default initialization provenance

§ 29(1) Semantic IR preserves the semantic reason for initialization.

§ 29(2) Provenance may include:

```text
explicit source initialization
implicit mutable default
omitted-field default
aggregate default
array-element default
empty collection default
explicit type default
range-derived default
compiler-generated canonical default
```

§ 29(3) All defaults come from canonical compiler-owned default resolution.

§ 29(4) Default lowering must never expose poison/undefined data as a readable Sec value.

---

## § 30 Storage locations

§ 30(1) Semantic IR distinguishes semantic SSA values from materialized storage locations.

§ 30(2) Storage locations may include:

```text
automatic/local
parameter/result storage
static
thread-local
field/element substorage
Arena-backed storage
allocator-backed storage
mapped storage
externally owned storage
MMIO/register storage
```

§ 30(3) Canonical storage classification follows `storage.md`.

§ 30(4) Local materialization does not imply dynamic allocation.

§ 30(5) Storage-domain identity and numeric address remain distinct.

---

## § 31 Storage operations

§ 31(1) Semantic IR supports distinctions equivalent to:

```text
EstablishStorageDomain
AdvanceStorageEpoch
EndStorageDomain
ConstructObject
EndObjectLifetime
InitializeStorage
Load
Store
Replace
Reinitialize
BindBackingStorage
RebindBackingStorage
RelocateStorage
AcquirePin
ReleasePin
AcquireReclamationProtection
ReleaseReclamationProtection
```

§ 31(2) Exact names are implementation details.

§ 31(3) Allocation, initialization, replacement, destruction, reclamation, relocation, and invalidation must not be collapsed into one generic store/free sequence before their semantics are resolved.

---

## § 32 Ownership classification

§ 32(1) Semantic IR distinguishes ownership/reference categories including:

```text
owned value
shared reference
mutable reference
RawPtr
non-owning immediate
compiler-owned temporary
resource/control token where applicable
```

§ 32(2) Ownership classification is distinct from physical storage ownership.

§ 32(3) Ownership must not be inferred later from low-level loads/stores.

---

## § 33 Availability

§ 33(1) The canonical ownership-availability dimension is:

```text
Uninitialized
Available
PartiallyAvailable
Unavailable
ConditionallyAvailable
```

§ 33(2) Unavailability reason/provenance is a separate fact.

§ 33(3) Reasons may include:

```text
Moved
Discarded
Detached
Destroyed
NeverInitialized
VariantInactive
```

§ 33(4) Control-flow joins may retain multiple possible reasons without collapsing the availability lattice.

§ 33(5) `ConditionallyAvailable` must not be represented as `Option`, nullability, or reference generation state.

---

## § 34 Copy

§ 34(1) A semantic copy duplicates a copyable value and leaves the source available.

§ 34(2) Copy operations are explicit until a semantics-preserving optimization eliminates them.

§ 34(3) Semantic IR must not contain a copy of a move-only value.

§ 34(4) A by-value non-consuming call argument uses copy semantics where the source requires copy.

---

## § 35 Move

§ 35(1) Move transfers ownership.

§ 35(2) A move identifies:

```text
source Place/value
destination/result
moved type
move-marker provenance where source syntax is relevant
commit behavior
source location
```

§ 35(3) After committed move, the source Place becomes unavailable.

§ 35(4) A move is not represented as an ordinary load when ownership changes.

§ 35(5) Physical relocation is not implied.

---

## § 36 Delayed ownership commit

§ 36(1) Semantic operations with multiple evaluated operands preserve Sec's transactional ownership behavior.

§ 36(2) For calls, arguments evaluate left-to-right.

§ 36(3) Transfer into consuming callee parameters commits only after all arguments have evaluated successfully and the call is ready to enter.

§ 36(4) Earlier temporaries/borrows must be cleaned correctly if a later argument fails.

§ 36(5) The same principle applies to constructors, aggregate formation, task/channel transfer, foreign transfer, and other multi-step consuming boundaries where their owning rules define a commit point.

---

## § 37 Partial ownership

§ 37(1) Semantic IR represents partial aggregate ownership at Place/sub-Place granularity.

§ 37(2) Moving a supported sub-Place leaves the aggregate `PartiallyAvailable`.

§ 37(3) Still-owned disjoint sub-Places remain available.

§ 37(4) Whole-value operations requiring full availability remain invalid.

§ 37(5) Destruction must skip already moved/discarded subobjects.

§ 37(6) Types whose canonical lifecycle forbids partial move must not receive partial-move IR.

---

## § 38 Conditional ownership

§ 38(1) Control-flow joins may produce `ConditionallyAvailable`.

§ 38(2) Availability tests may refine the state on branches.

§ 38(3) Semantic IR preserves enough state to lower conditional destruction/reinitialization where the selected policy permits it.

§ 38(4) A target/profile that forbids dynamic availability bookkeeping may reject an unprovable source program before lowering.

---

## § 39 Discard

§ 39(1) Validated discard is an explicit terminal ownership action.

§ 39(2) The canonical conceptual operation is:

```text
DiscardValue
```

§ 39(3) It records at least:

```text
consumed Place/value
type
destruction/trivial classification
resulting availability state
discard provenance
source location
```

§ 39(4) Discard provenance distinguishes at least explicit source discard, legal implicit call-result discard, and compiler temporary cleanup.

§ 39(5) Unavailable discard is a legal no-op where ownership rules define convergence.

§ 39(6) Lower stages must not infer semantic discard from an unused SSA result.

---

## § 40 Replacement and reinitialization

§ 40(1) Assignment to an Available mutable Place is semantic replacement.

§ 40(2) Replacement performs required cleanup of the old value before the new lifetime is established.

§ 40(3) Assignment to an Unavailable mutable Place is reinitialization.

§ 40(4) Reinitialization performs no cleanup for an absent old value.

§ 40(5) Conditional availability may require conditional cleanup.

§ 40(6) Semantic IR must preserve the distinction until cleanup/ownership semantics are fully materialized.

---

## § 41 Construction

§ 41(1) Lifecycle construction remains distinct from conversion.

§ 41(2) A resolved construction retains:

```text
target type
selected explicit init or implicit constructor path
arguments
ownership actions
exact construction error type
partial-initialization state
cleanup plan
effects
source provenance
```

§ 41(3) Success produces exactly one fully initialized target value.

§ 41(4) Failure cleans only initialized/acquired state.

§ 41(5) Completed-value custom `free` must not run on failed partial construction.

§ 41(6) `new Type(...)` by itself must not imply hidden dynamic allocation.

---

## § 42 Destruction

§ 42(1) Required deterministic destruction is explicit or represented through a canonical cleanup plan.

§ 42(2) Destruction identifies:

```text
Place/value
type
destruction plan
trivial/nontrivial classification
conditional/partial state
source/synthesized provenance
```

§ 42(3) Each owned value requiring destruction is destroyed exactly once unless ownership is transferred/discarded according to the owning rule.

§ 42(4) Observable destruction order must not be reordered.

---

## § 43 Cleanup plans

§ 43(1) Cleanup behavior is explicit on every relevant exit edge.

§ 43(2) Cleanup may include:

```text
local destruction
defer execution
temporary cleanup
resource release
error propagation cleanup
return cleanup
break/continue cleanup
match/switch arm cleanup
task/thread completion cleanup
compiler-known lifecycle termination
```

§ 43(3) Cleanup plans preserve registration/source ordering required by Sec.

§ 43(4) Cleanup must not depend on LLVM exception handling unless a canonical Sec/FFI rule explicitly requires such a mechanism.

---

## § 44 Defer

§ 44(1) Every `defer` is represented explicitly.

§ 44(2) Semantic IR preserves:

```text
registration order
captured values
capture modes
owning execution scope
cleanup registration identity
effects
borrows/lifetimes
```

§ 44(3) Deferred cleanup and automatic destruction participate in the common registration-time LIFO order defined by `defer`/destruction rules.

§ 44(4) A defer inside a lambda belongs to that lambda invocation.

§ 44(5) `free` cannot contain defer in Sec 0.1; such IR must not be generated.

---

## § 45 References

§ 45(1) Safe references retain or reference canonical facts from `reference_model.md`.

§ 45(2) Relevant facts include:

```text
reference category
referenced type
source storage identity
origin/provenance
bounds
borrow authority
validity epoch dependency
address space
mapping/platform dependency
source location
```

§ 45(3) Semantic IR does not require source-visible lifetime parameters.

§ 45(4) Reference facts may be reduced only after every downstream consumer has equivalent proof.

---

## § 46 Borrow operations

§ 46(1) Borrow creation/reborrow remains explicit while ownership/alias/lifetime-sensitive transformations depend on it.

§ 46(2) Conceptual operations include:

```text
BorrowShared
BorrowMutable
ReborrowShared
ReborrowMutable
EndBorrowDependency
```

§ 46(3) Borrow operations are generated only after validation.

§ 46(4) The Semantic IR verifier need not rerun the full borrow solver but must reject structurally contradictory borrow state.

§ 46(5) NLL/final-use facts may be retained as explicit dependency endpoints or equivalent metadata.

---

## § 47 Raw pointers

§ 47(1) `RawPtr[T]` operations remain distinct from safe references.

§ 47(2) Semantic IR supports explicit distinctions for:

```text
raw construction
raw read/write
volatile raw read/write
Offset
AddBytes
Difference
safe-reference to raw conversion
raw to safe-reference conversion
raw/integer conversion
raw equality
```

§ 47(3) Unsafe/trust provenance and target address-space facts remain attached.

§ 47(4) Raw-pointer operations do not imply ownership, non-nullness, lifetime, bounds, or safe provenance.

---

## § 48 Reference invalidation and handles

§ 48(1) Semantic IR represents invalidation events where reference correctness depends on them.

§ 48(2) Events may include:

```text
allocation end
Arena Reset
Arena Release
collection backing replacement
slot reuse/removal
mapping remap/unmap
owner-domain retirement
```

§ 48(3) Direct references, stable handles, weak handles, and RawPtr remain semantically distinct.

§ 48(4) Stale ordinary safe-reference failure remains panic/trap behavior when dynamically checked.

§ 48(5) Stale stable/weak-handle resolution remains fallible.

---

## § 49 Allocation operations

§ 49(1) Allocation remains an explicit semantic operation/effect.

§ 49(2) Semantic IR records:

```text
allocation domain/context
provider/origin where resolved
requested type/size/count
alignment/layout
failure channel
ownership/result relationship
effects
source location
```

§ 49(3) Copy, move, borrow, reference creation, return, and escape repair must not silently synthesize allocation.

§ 49(4) Allocation-context requirement is distinct from `MayAllocate`.

---

## § 50 Arena representation

§ 50(1) Arena semantics remain explicit until Arena-specific ownership, epoch, dependency, capacity, effects, and physical planning are complete.

§ 50(2) Semantic IR distinguishes:

```text
Arena owner state
ArenaDomain identity
Arena state version
Arena validity epoch
backing/provider policy
growth policy
allocation context
Arena dependency
capacity proof
```

§ 50(3) Arena state version and validity epoch are distinct facts.

§ 50(4) Ordinary Arena allocation advances state version but not epoch.

§ 50(5) `Reset()` advances state version and validity epoch while preserving ArenaDomain.

§ 50(6) `Release()` consumes Arena owner state and terminates ArenaDomain.

---

## § 51 Arena operations

§ 51(1) Semantic IR supports distinctions equivalent to:

```text
ArenaCreateBorrowed
ArenaCreateOwnedFixed
ArenaCreateGrowable
ArenaNew
ArenaAlloc
ArenaReset
ArenaRelease
ArenaDestroy
```

§ 51(2) Allocation success/failure produces one continuing Arena state, never duplicate owners.

§ 51(3) Failure leaves the physical Arena state equivalent to input according to Arena atomicity rules.

§ 51(4) Growable Arena planning must preserve prior live allocation addresses.

§ 51(5) Arena dependency across task/thread/FFI/result boundaries remains explicit or canonically recoverable.

---

## § 52 Effects

§ 52(1) Semantic IR retains compiler-owned effects and verified guarantees from `effect_analysis.md`.

§ 52(2) Effects are not inferred later from operation names.

§ 52(3) Relevant effects may include:

```text
MayPanic
MayAllocate
MayBlock
MaySuspend
MaySpawn
MayIO
MayAccessVolatile
MayMutateExternalState
MayUseNondeterministicInput
```

and other canonical effect identities.

§ 52(4) Effects may be attached to operations, calls, functions, generated helpers, cleanup, and execution roots.

§ 52(5) Verified guarantees such as `noPanic`, `noAlloc`, and `noBlock` remain distinguishable from absence of locally observed effect sites.

§ 52(6) Unsafe does not suppress effects.

---

## § 53 Ordered effects

§ 53(1) Effects whose order is semantically observable remain ordered.

§ 53(2) Examples include:

```text
volatile accesses
hardware register transactions
Arena create/allocate/reset/release
synchronization
FFI/syscalls
external I/O
cleanup/destruction with effects
panic checks
```

§ 53(3) An unordered summary is insufficient when operation order changes meaning.

---

## § 54 Runtime checks

§ 54(1) Required runtime checks are explicit or introduced by a defined semantic lowering.

§ 54(2) Checks may include:

```text
bounds
narrowing conversion
division by zero
invalid shift
overflow
enum/representation validity
raw/FFI null or alignment validation
reference epoch/generation
shape compatibility
hardware mapping/access validation
assertion
```

§ 54(3) Each check retains:

```text
condition
failure mode
stable reason/error identity
source location
effect contribution
```

§ 54(4) A check may be removed only after proof it cannot fail.

---

## § 55 Panic

§ 55(1) Panic-producing semantic operations are explicit in control/effect semantics.

§ 55(2) Panic is distinct from `Result` propagation.

§ 55(3) Semantic IR preserves where relevant:

```text
panic reason identity
source/check origin
static message or message identity
panic domain/context
containment boundary
cleanup policy requirements
noPanic effect facts
```

§ 55(4) Lowering must not silently convert panic into `Err`.

§ 55(5) Lowering must not invent exception unwinding when the selected panic policy does not provide it.

---

## § 56 Assertion

§ 56(1) Validated source forms:

```sec
assert condition
assert condition, "message"
```

lower to explicit assertion/check semantics.

§ 56(2) The condition is already typed `bool`.

§ 56(3) A proven-true assertion may be eliminated and may refine analysis facts.

§ 56(4) A dynamically failing assertion produces the canonical assertion panic reason.

§ 56(5) Assertion message is the validated static/string-literal diagnostic payload defined by panic rules.

§ 56(6) `assert` must not be represented as optimizer-only `assume`.

---

## § 57 Checked unreachable

§ 57(1) Source/compiler-known checked unreachable semantics remain defined panic behavior unless Sema proves the edge impossible.

§ 57(2) Backend `unreachable` may be emitted only after:

```text
a non-returning Sec panic/termination operation
or proof that the edge is semantically impossible
```

§ 57(3) Compiler-generated exhaustive residual blocks may use a synthesized semantic unreachable reason before later proof/lowering.

§ 57(4) Semantic IR must not expose backend UB for a source path that Sec defines as checked failure.

---

## § 58 Arithmetic

§ 58(1) Arithmetic operations retain resolved Sec arithmetic mode.

§ 58(2) Semantic IR distinguishes:

```text
signed/unsigned integer
decimal
floating point
unit-aware
checked
wrapping where explicitly defined
saturating where explicitly defined
```

§ 58(3) Checked builtin integer arithmetic may produce conceptual:

```text
result
failed
reason: ArithmeticFailureReason
```

§ 58(4) Failure reasons preserve at least canonical overflow, division-by-zero, and invalid-shift identities where applicable.

§ 58(5) Backend default overflow/UB behavior must not redefine Sec arithmetic.

---

## § 59 Conversions

§ 59(1) Every non-identity semantic conversion is explicit.

§ 59(2) Conversion classes include:

```text
numeric widening/narrowing
signedness
integer/float
named/distinct type
unit
enum
reference/raw pointer
ABI
representation reinterpretation
address-space conversion
```

§ 59(3) A conversion retains source/target type, validation mode, runtime-check requirement, unsafe/trust status, and source provenance.

§ 59(4) Lower stages may not invent implicit conversions.

---

## § 60 Function representation

§ 60(1) A Semantic IR function retains at least:

```text
symbol identity
source/display name
linker identity where applicable
parameters
parameter ownership modes
receiver metadata where relevant
return type/ownership mode
unsafe-callable status
calling convention
generic specialization identity
interface/effect contracts
attributes
entry block/CFG
cleanup plan
source provenance
```

§ 60(2) Parameters preserve by-value, shared-borrow, mutable-borrow, and explicit consuming mode.

§ 60(3) Receiver syntax may be normalized to an explicit parameter while preserving receiver capability metadata.

---

## § 61 Calls

§ 61(1) A call operation identifies a resolved callable target or a resolved indirect-call contract/target set.

§ 61(2) It retains:

```text
target identity
calling convention
arguments
argument ownership actions
return type/action
unsafe/FFI status
effect summary/cause
error/panic behavior
commit point
source location
```

§ 61(3) Default/named arguments and method syntax are already resolved before Semantic IR.

§ 61(4) Borrowed arguments use validated references/reborrows.

---

## § 62 Indirect calls

§ 62(1) Semantic IR distinguishes at least:

```text
function value call
interface dispatch
closure call
extern callback call
resolved finite-target indirect call
open-contract indirect call
```

§ 62(2) Indirect calls retain the resolved callable type, ownership modes, effect contract, unsafe status, and target-set/summary provenance.

§ 62(3) Unknown/open target behavior remains conservative according to the owning analysis rulebooks.

---

## § 63 Interfaces

§ 63(1) Semantic IR preserves interface identity and explicit conformance relationships needed after Sema.

§ 63(2) A resolved interface call must not rerun member/conformance lookup.

§ 63(3) Interface method calls retain:

```text
interface identity
concrete implementation identity where statically known
resolved method identity
receiver capability
argument ownership contract
result/effects
dispatch class
```

§ 63(4) A generic interface contract remains parameterized until concrete specialization/conformance is resolved.

§ 63(5) Interface representation must not silently imply one universal runtime vtable when static resolution is sufficient.

---

## § 64 Compiler-known `Iterator[T]`

§ 64(1) `Iterator[T]` is a compiler-known generic interface used by the canonical Sec iteration protocol.

§ 64(2) Semantic IR represents an iterator loop only after Sema has resolved:

```text
concrete iterator value/type
concrete `Iterator[T]` conformance
yielded T
resolved `Next()` implementation
ownership/borrow mode of iterator state
termination via Option[T]
effects of `Next()`
source loop-binding mode
```

§ 64(3) Iterator discovery by naming convention must not occur in Semantic IR.

§ 64(4) Semantic IR must not require a runtime dynamic-dispatch object solely because the protocol is expressed as an interface.

§ 64(5) When the concrete iterator/conformance is statically known, `Next() Option[T]` is represented as a statically resolved call or equivalent canonical iterator-step operation.

§ 64(6) Compiler-known collection/range/string iteration may specialize directly, but the resulting semantics must remain equivalent to the resolved iteration contract.

§ 64(7) Lower stages must not reintroduce a closed whitelist of user-visible iterable types as the language rule.

---

## § 65 `for` iteration

§ 65(1) Source `for` is normalized to explicit loop CFG plus resolved iteration semantics.

§ 65(2) Semantic IR distinguishes:

```text
infinite for
numeric range iteration
compiler-known specialized iteration
Iterator[T]-based iteration
```

only as needed for semantics/lowering.

§ 65(3) Loop bindings preserve:

```text
by-value copy
shared borrow
mutable borrow
discard
```

according to the resolved iteration contract.

§ 65(4) Ordinary iteration does not silently consume collection elements.

§ 65(5) Iterator state lifetime/destruction is explicit.

§ 65(6) Each `Next()` result is tested as `Option[T]`; `None` exits the loop and `Some` produces the next yielded value.

---

## § 66 Loop CFG

§ 66(1) Loops have explicit:

```text
entry
condition/step
body
continue target
break target
exit
loop-carried values/state
cleanup edges
```

§ 66(2) Loop-carried ownership/availability state must be unambiguous.

§ 66(3) A Place moved in one iteration is unavailable in the next unless reinitialized on every continuing path.

§ 66(4) Iterator state itself may be loop-carried SSA state when its `Next()` contract mutates logical iterator position.

---

## § 67 `if` and `while`

§ 67(1) `if` becomes explicit conditional control flow and merge state.

§ 67(2) Value-producing branches merge through typed block parameters or equivalent.

§ 67(3) Ownership/availability/cleanup state must join consistently.

§ 67(4) `while` becomes explicit condition/body/continue/exit CFG with canonical bool condition.

---

## § 68 `switch`

§ 68(1) `switch` semantics are resolved before lowering.

§ 68(2) Semantic IR may use a semantic switch operation or explicit comparisons/branches.

§ 68(3) Case ordering/overlap/range semantics are not rediscovered by the backend.

§ 68(4) Branch cleanup/ownership state remains explicit.

---

## § 69 `match`

§ 69(1) Resolved `match` is represented as explicit CFG or an equivalent form preserving identical semantics.

§ 69(2) The subject is evaluated exactly once.

§ 69(3) Semantic IR preserves:

```text
subject Place/value identity
source arm order
pattern kind
resolved variant/member/value identity
bindings
binding actions
candidate guard state
guard-false continuation
guard-success ownership commit
branch-scoped borrow begin/end
arm result
cleanup
post-arm availability
exhaustiveness proof
```

§ 69(4) Binding actions include the canonical copy/move/borrow/discard/temporary-forward categories resolved by Sema.

§ 69(5) Backend must not perform match exhaustiveness analysis.

§ 69(6) Guarded move-only bindings commit ownership only on selected guard-success edges.

---

## § 70 Exhaustive residual paths

§ 70(1) A compiler-proven exhaustive residual path may terminate with a synthesized semantic unreachable operation.

§ 70(2) The operation retains reason/provenance such as `exhaustive-match-fallthrough`.

§ 70(3) It becomes backend unreachable only after Semantic IR verification establishes the proof.

---

## § 71 `try`

§ 71(1) `try` consumes an immutable Sema-resolved handler plan.

§ 71(2) Semantic IR preserves:

```text
source handler order
specific/catch-all pattern identity
handler-local binding type
where guard
guard-false continuation
binding copy/move/borrow action
guard-success ownership commit
recovery-value merge
unmatched propagation
terminating/returning handler flow
cleanup on every edge
```

§ 71(3) Success alternatives are implicit according to the owning fallible type/operation and do not become user-specified Ok/Some handler arms.

§ 71(4) `try` does not catch panic.

§ 71(5) Checked arithmetic may enter the same resolved error-handler machinery without constructing an unnecessary temporary `Result`.

---

## § 72 Result propagation

§ 72(1) Error propagation is explicit control flow.

§ 72(2) Propagation executes required cleanup before returning/forwarding failure.

§ 72(3) `return try expr` is represented as the appropriate resolved success/failure forwarding semantics.

§ 72(4) Fallible property assignment and other language-defined fallible operations retain explicit failure edges.

§ 72(5) Failure sets used internally must not become anonymous inferred source error unions.

---

## § 73 Function return

§ 73(1) Return operations provide explicit resolved return value where applicable.

§ 73(2) Required cleanup occurs before control leaves the function, while preserving the returned value/ownership transfer.

§ 73(3) Owned return transfers ownership to caller.

§ 73(4) Returning a reference preserves validated origin/lifetime/storage relationship.

§ 73(5) Optional source move marker on terminal return may be retained only as provenance; semantic ownership transfer is the same where both source forms are legal.

---

## § 74 Closures and lambdas

§ 74(1) Semantic IR preserves:

```text
lambda source identity
concrete callable signature
callable receiver/capability if applicable
captures
capture modes
capture types
environment ownership/copy/move
escape class
environment lifetime
construction
invocation
consumption
destruction
effects
allocation requirement if materialization allocates
```

§ 74(2) Capture modes include owned copy, owned move, shared borrow, mutable borrow, and canonical static/raw/foreign forms.

§ 74(3) Semantic IR must not force every closure to one universal `{code_ptr, env_ptr}` representation.

§ 74(4) Escaping closure allocation remains explicit and must not be introduced to repair illegal lifetime.

---

## § 75 Generics

§ 75(1) Semantic IR retains generic declarations/templates only when required by the selected specialization pipeline.

§ 75(2) Concrete specialization identity is compiler-owned and deterministic.

§ 75(3) A concrete specialization retains:

```text
generic declaration identity
concrete type/value arguments
resolved constraints/conformances
CompilationPlan-sensitive facts where applicable
specialized callable/type identity
```

§ 75(4) Generic specialization must not erase ownership, effect, interface, layout, storage, or target constraints.

§ 75(5) Exact monomorphization/lowering policy is owned by the corresponding generic compiler rulebooks.

---

## § 76 Collection/iterator specialization

§ 76(1) Generic algorithms constrained by `Iterator[T]` preserve resolved conformance in Semantic IR.

§ 76(2) Monomorphized code may statically bind `Next()` to the concrete iterator implementation.

§ 76(3) The optimizer may inline/specialize iterator steps after semantic resolution.

§ 76(4) This optimization does not make `Iterator[T]` a naming convention or a compiler-only closed list of iterable nominal types.

---

## § 77 Tasks

§ 77(1) Task operations remain explicit semantic concurrency operations until execution/lifecycle/ownership semantics are lowered.

§ 77(2) Semantic IR represents distinctions equivalent to:

```text
TaskSpawn
TaskAwait
TaskJoin
TaskCancelRequest
TaskOutcome
TaskDetach where canonical
```

§ 77(3) Task creation is fallible where canonical concurrency rules define it.

§ 77(4) Spawn captures/arguments retain copy/move/borrow transfer actions and commit behavior.

§ 77(5) Task completion occurs only at the canonical semantic completion point after required result transfer and cleanup.

§ 77(6) Await/join result/dependency transfer remains explicit.

---

## § 78 Threads

§ 78(1) Physical thread operations remain distinct from tasks.

§ 78(2) Semantic IR preserves:

```text
thread creation
join/completion
ownership transfer
thread-affinity facts
thread-local dependency
result/outcome
blocking effects
target/runtime requirements
```

§ 78(3) Task migration and physical-thread affinity must remain distinguishable where relevant.

§ 78(4) Thread-local references/capabilities retain physical-thread dependency.

---

## § 79 Transfer boundaries

§ 79(1) Transferability-validated boundaries retain enough facts to distinguish:

```text
owned task transfer
borrowed task transfer
owned thread transfer
borrowed thread transfer
process adapter transfer
ISR handoff
foreign callback-context transfer
```

§ 79(2) Exclusive ownership transfer is not concurrent sharing.

§ 79(3) Process transfer is not ordinary pointer/value bit-copy unless the process/IPC contract explicitly defines such representation.

§ 79(4) Lowering must preserve source ownership commit/failure semantics.

---

## § 80 Channels and synchronization

§ 80(1) Channel send/receive/select and synchronization operations remain explicit where order/ownership/blocking semantics matter.

§ 80(2) A consuming send commits ownership according to the channel contract.

§ 80(3) Mutex/guard operations retain scoped capability semantics.

§ 80(4) Atomic operations retain memory-order/target requirements defined by concurrency rules.

§ 80(5) Volatile access must not be lowered as synchronization.

§ 80(6) Semantic IR and concurrency analysis share canonical memory-location identity.

---

## § 81 Cancellation and structured concurrency

§ 81(1) Cancellation request is distinct from task/thread completion.

§ 81(2) Structured-concurrency scopes preserve child lifecycle and cleanup obligations.

§ 81(3) A scope may discharge dependencies only after canonical child completion and result/dependency handling.

§ 81(4) Detached execution remains an explicit escape/transferability boundary.

---

## § 82 Thread-local storage

§ 82(1) Thread-local storage retains one physical-thread domain identity.

§ 82(2) Task migration does not move thread-local storage identity.

§ 82(3) References into thread-local storage carry affinity dependency.

§ 82(4) Semantic IR must not treat thread-local as ordinary process-global static storage.

---

## § 83 Platform operations

§ 83(1) Platform-specific operations remain explicit.

§ 83(2) Examples include:

```text
raw syscalls
fixed-address access
target intrinsics
interrupt operations
architecture-specific instructions
platform error access
runtime mappings
```

§ 83(3) Each operation retains selected-target applicability and semantic contract.

§ 83(4) Unsupported selected-target operations are rejected before incompatible lowering.

---

## § 84 Volatile access

§ 84(1) Volatile read/write are explicit semantic operations.

§ 84(2) Semantic IR retains:

```text
external storage identity
access width/type
address space
ordering constraints
physical access contract
effects
source provenance
```

§ 84(3) Volatile read produces an ordinary detached snapshot value after the physical access.

§ 84(4) Copy/move of that snapshot must not repeat the physical access.

§ 84(5) Volatile remains distinct from atomics/synchronization.

---

## § 85 Hardware register operations

§ 85(1) Hardware register accesses remain explicit semantic operations or immutable verified plans until target-aware lowering materializes them.

§ 85(2) Semantic IR preserves at least:

```text
logical register operation
hardware resource identity
endpoint/physical operation identity
selected physical plan
read/write projections
transaction footprints
shadow observations/invalidations/pending writes
specialized field effects
compiler ordering
hardware ordering/completion
access-context requirements
fault behavior
```

§ 85(3) Lowering must not reconstruct these facts from address/width/naming convention.

---

## § 86 Interrupts and ISR roots

§ 86(1) Resolved interrupt bindings retain canonical named/logical interrupt identities and target binding facts.

§ 86(2) ISR entry remains distinct from ordinary callable entry.

§ 86(3) Semantic IR preserves generated claim/dispatch/completion/wrapper operations where the selected platform requires them.

§ 86(4) `@isr`/`@interruptSafe` constraints preserve transitive `noPanic`, `noAlloc`, and `noBlock` proof.

§ 86(5) Unsafe does not erase ISR context requirements.

§ 86(6) Interrupt return must not bypass required ordinary cleanup.

---

## § 87 FFI calls

§ 87(1) Extern calls retain:

```text
foreign symbol identity
ABI/calling convention
safe versus unsafe-extern caller contract
ABI parameter/return types
ownership/retention/nullability
foreign type identity
varargs/callback adaptation
effect/trust provenance
target restrictions
native dependency identity
```

§ 87(2) An extern call does not automatically create `Result`, `Option`, or ownership semantics.

§ 87(3) Safe wrappers remain ordinary Semantic IR around the extern operation.

§ 87(4) Foreign retention/callback-thread facts remain explicit where needed by lifetime/transferability.

---

## § 88 Unsafe operations

§ 88(1) Unsafe operations remain marked with operation identity and trust provenance.

§ 88(2) Unsafe metadata supports verification, auditing, diagnostics, and generated-code inspection.

§ 88(3) Unsafe does not alter ordinary ownership/type/effect rules.

§ 88(4) A compiler-proven invalid operation must not become valid IR merely because the source used `unsafe`.

§ 88(5) Safe-wrapper boundaries may retain internal trust provenance without making callers unsafe.

---

## § 89 Inline assembly boundary

§ 89(1) Inline assembly remains a semantic trust/effect boundary until the canonical inline-assembly rulebook lowers its complete contract.

§ 89(2) Semantic IR must preserve at least the information supplied by that rulebook for:

```text
inputs/outputs
register constraints
clobbers
memory access
volatile/ordering behavior
control flow
stack behavior
possible trap/abort
blocking/I/O/external mutation
target applicability
```

§ 89(3) This rulebook does not define inline-assembly source syntax.

---

## § 90 Source locations

§ 90(1) Every user-originating operation should retain a source location.

§ 90(2) Synthesized operations retain:

```text
originating source construct
synthesized marker
optional reason/provenance
```

§ 90(3) Source mapping must survive Semantic IR transformations and lowering as required by diagnostics/debug information.

§ 90(4) Diagnostics prefer the original user location over synthetic helper locations.

---

## § 91 Semantic plans versus AST

§ 91(1) Semantic IR builders consume immutable resolved semantic plans/facts rather than mutating/requerying AST where a canonical plan exists.

§ 91(2) Examples include:

```text
ResolvedMatchPlan
ResolvedStructLiteralPlan
ResolvedStructMemberPlan
resolved try-handler plan
resolved call/ownership commit plan
resolved iterator plan
resolved hardware register plan
resolved allocation/Arena plan
```

§ 91(3) Exact plan type names are implementation details.

§ 91(4) AST nodes must not be used as the canonical identity of a Semantic IR operation.

---

## § 92 Resolved iterator plan

§ 92(1) Sema should provide an immutable resolved iteration plan or equivalent facts.

§ 92(2) The plan records:

```text
iteration category
source expression identity
iterator construction/selection if any
Iterator[T] conformance identity where used
resolved Next() target
yield type
loop binding modes
iterator mutation/ownership mode
structural-stability dependency
effects
cleanup/destruction
source location
```

§ 92(3) Semantic IR generation must not discover iteration by probing method names.

§ 92(4) Iterator protocol lowering must be deterministic across compiler/LSP builds.

---

## § 93 Basic blocks

§ 93(1) Semantic IR control flow consists of basic blocks or equivalent structured regions.

§ 93(2) Each block contains:

```text
BlockID
block parameters
ordered operations
exactly one terminator
```

§ 93(3) Fallthrough is not implicit.

§ 93(4) Terminators include semantic branch, conditional branch, switch, return, propagation, panic/termination, and verified unreachable forms as needed.

---

## § 94 Branch arguments

§ 94(1) Values/state crossing block boundaries use block arguments or an equivalent explicit mechanism.

§ 94(2) Branch arguments match destination types.

§ 94(3) Ownership/availability/storage state must also join compatibly.

§ 94(4) Mutually exclusive branches may carry the same pre-branch owner to different successors, but a join must not create duplicate live owners.

§ 94(5) Conditional cleanup/drop state must join conservatively.

---

## § 95 Function-local semantic records

§ 95(1) Semantic IR may retain function-local records for complex source constructs after CFG expansion.

§ 95(2) Records may support diagnostics/tooling for:

```text
match arms
iterator loops
try handlers
closure captures
Arena operations
spawn/call transfers
hardware plans
```

§ 95(3) Such records retain semantic identity/provenance and must not retain AST as authority.

---

## § 96 Semantic verifier

§ 96(1) Semantic IR has a dedicated verifier.

§ 96(2) The verifier checks structural and semantic consistency without rerunning all source analyses.

§ 96(3) It must reject at least:

```text
unresolved/invalid types
invalid operands/results
invalid block targets/terminators
branch argument mismatch
ownership duplication
use after move/discard/release
invalid whole-value use of partial availability
invalid conditional state
duplicate/missing destruction
invalid initialization
invalid reference origin/dependency
invalid Arena state/domain/epoch sequence
invalid call/receiver/iterator target
invalid Result/Option projection
invalid error/try flow
invalid effect/guarantee contradiction
invalid panic/check reason
invalid target/platform applicability
invalid transfer/synchronization state
invalid source/synthetic provenance where required
```

§ 96(4) Verifier failures are internal compiler errors.

---

## § 97 Verification schedule

§ 97(1) Verification runs:

```text
after initial Semantic IR generation
after each Semantic IR transformation in debug/assertion compiler builds
before Sec MLIR lowering
before serialization/caching when required
after deserialization/cache load
```

§ 97(2) A release compiler may reduce intermediate verifier frequency but must not skip required boundary verification.

---

## § 98 Semantic IR transformations

§ 98(1) Semantic IR transformations preserve Sec semantics.

§ 98(2) Permitted examples include:

```text
control-flow canonicalization
cleanup insertion/normalization
try/Result propagation expansion
match CFG canonicalization
iterator-loop canonicalization
ownership-state normalization
constant folding using Sec semantics
unreachable-block removal after proof/diagnostics
trivial copy elimination
proven check elimination
generic specialization
Arena capacity/check specialization
reference-check elimination after proof
```

§ 98(3) Transformations may become target-plan-aware only when explicitly classified and must preserve target-independent semantic contracts.

---

## § 99 Forbidden semantic transformations

§ 99(1) A transformation must not without proof:

```text
alter evaluation order
alter observable cleanup/destruction order
introduce hidden allocation
introduce hidden ownership transfer
change allocation domain/provider
remove required check
change Result/panic behavior
change unsafe/trust boundary
convert fallible handle resolution to trap
convert panic to Err
reconstruct iterator discovery by method name
replace Iterator[T] semantics with a closed user-visible iterable whitelist
relocate live Arena allocations during growth
erase storage/reference/ownership facts before downstream consumers finish
introduce stronger backend alias/lifetime assumptions
```

---

## § 100 Optimization boundary

§ 100(1) Semantic IR optimization focuses on transformations requiring Sec semantic knowledge.

§ 100(2) General machine-oriented optimization belongs primarily in Sec MLIR/standard MLIR/LLVM.

§ 100(3) Semantic simplification may use ownership, type, effect, reference, iterator, error, Arena, or platform proofs.

§ 100(4) Observable Sec behavior must remain unchanged.

---

## § 101 Observable behavior

§ 101(1) Observable behavior includes where applicable:

```text
returned values
externally visible writes
volatile/MMIO transactions
FFI/syscalls
errors/Result propagation
panic/termination behavior
allocation failure/domain behavior
destruction/defer order
synchronization/atomic effects
task/thread lifecycle
hardware ordering/completion
inline assembly effects
required runtime checks
```

§ 101(2) Optimizers may remove unobservable implementation artifacts only after proof.

---

## § 102 Lowering boundary

§ 102(1) Semantic IR lowering to Sec MLIR occurs only when every semantic distinction needed by lower stages is either explicitly represented or proven irrelevant.

§ 102(2) Lowering must not:

```text
re-run source name lookup
re-run overload/member resolution
invent iterator conformance
invent ownership actions
invent allocation
invent panic/error channels
invent target ABI semantics
weaken reference/storage safety
```

§ 102(3) Lowering may select physical representation from plan-resolved facts.

---

## § 103 Sec MLIR relationship

§ 103(1) Sec MLIR is a lowering representation, not the source of language semantics.

§ 103(2) High-level Sec MLIR may preserve semantic operations such as:

```text
ownership/move/copy
struct/union/Result/Option operations
Arena state/domain operations
checked arithmetic
reference/storage operations
task/thread operations
hardware-register operations
panic/check operations
```

§ 103(3) The exact Sec dialect schema/version is implementation-governed.

§ 103(4) Semantic IR may support a concept before Sec MLIR has a corresponding executable lowering; governance tracks that gap.

---

## § 104 Layout boundary

§ 104(1) Target-independent Semantic IR may retain unresolved plan-sensitive layout requirements.

§ 104(2) Before layout-sensitive lowering, a concrete `CompilationPlan` provides canonical `ResolvedLayout` facts.

§ 104(3) Semantic IR types remain semantic even after attaching resolved layout identity.

§ 104(4) LLVM/MLIR data layout is not a competing Sec layout authority.

---

## § 105 Incremental compilation

§ 105(1) Semantic IR structure should support future/current incremental compilation.

§ 105(2) Potential cache boundaries include:

```text
resolved module interface
canonical type declarations
conformance/specialization summaries
validated function Semantic IR
resolved semantic plans
target-plan-specific derived facts
```

§ 105(3) Cached Semantic IR includes sufficient schema/rulebook/compiler/target/profile dependency identity to reject incompatible reuse.

§ 105(4) Incremental reuse must not weaken verification.

---

## § 106 Versioning

§ 106(1) Semantic IR has an internal schema/version.

§ 106(2) The version changes when incompatible operation/type/invariant/serialization semantics change.

§ 106(3) The compiler rejects incompatible serialized Semantic IR.

§ 106(4) No long-term external compatibility guarantee is required before the format is declared stable.

---

## § 107 Determinism

§ 107(1) Equivalent source plus equivalent `CompilationPlan` must produce deterministic semantic identities/order where those artifacts are persisted, tested, or compared.

§ 107(2) Determinism includes:

```text
symbol/type/value IDs where stable form exists
generic specialization identities
interface/conformance resolution
iterator plan identity
effect ordering
generated platform artifact identities
serialized record order
diagnostic cause ordering
```

§ 107(3) Compiler-host map iteration, address layout, or thread scheduling must not determine semantic output.

---

## § 108 Textual debug form

§ 108(1) The compiler should expose a readable textual Semantic IR form.

§ 108(2) It may show:

```text
modules
types
symbols
functions
blocks
values
Place/ownership state
cleanup
effects
references/storage
iterator/Arena/concurrency operations
source locations
```

§ 108(3) The textual form is compiler/debugging syntax, not Sec source syntax.

§ 108(4) It must not be accepted as user Sec source.

---

## § 109 LSP and tooling

§ 109(1) LSP, `sec analyse`, compiler diagnostics, and IR inspection consume the same canonical semantic facts.

§ 109(2) Tooling must not implement a second iterator/interface/ownership/reference/effect model.

§ 109(3) Tooling may expose:

```text
resolved target/conformance
Iterator[T] resolution
ownership/availability transition
borrow/reference origin
effect cause
allocation/Arena domain
panic/check reason
task/thread transfer
hardware plan
source-to-synthesized operation mapping
```

§ 109(4) Tooling facts belong to one compilation snapshot/plan.

---

## § 110 Diagnostics

§ 110(1) Semantic IR itself is not normally the user-facing diagnostic stage, but it preserves enough provenance to explain validated semantic plans and lowering failures.

§ 110(2) Internal verifier diagnostics identify operation identity, semantic invariant, source/synthetic provenance, and relevant canonical IDs.

§ 110(3) When surfaced to users, diagnostics follow the mentor-compiler principle and point back to source constructs rather than exposing internal op names unnecessarily.

---

## § 111 Implementation boundary

§ 111(1) The Semantic IR implementation belongs in a dedicated compiler subsystem.

§ 111(2) It provides or references canonical:

```text
type records
symbol/value/Place identities
operation definitions
module/function/block records
builder APIs
verifier
printer/serializer
semantic-plan bridge
transform interfaces
traversal utilities
```

§ 111(3) AST nodes are not reused as Semantic IR nodes.

§ 111(4) LLVM types/instructions are forbidden from becoming Semantic IR representation dependencies.

§ 111(5) Builder APIs require resolved types and validated semantic plans/operands.

---

## § 112 Required test families: core

§ 112(1) Required tests include:

```text
module/symbol/type/value identity
named/distinct type preservation
constants/target-sized constants
struct plans/spread/default provenance
arrays/slices/strings
enums/unions/Result/Option
properties/static members
units/registers
```

§ 112(2) Tests verify deterministic printing and verifier rejection of malformed records.

---

## § 113 Required test families: ownership and memory

§ 113(1) Required tests include:

```text
copy
move
delayed transfer commit
partial move
conditional availability
is available refinement
discard convergence
replacement
reinitialization
construction failure cleanup
destruction
defer/common LIFO cleanup
reference/borrow origin
raw pointer operations
storage epoch/invalidation
Arena create/alloc/reset/release
```

§ 113(2) No lower representation may reconstruct these semantics from dead/unused SSA values.

---

## § 114 Required test families: control/error/panic

§ 114(1) Required tests include:

```text
if/while/switch CFG
match one-evaluation and guarded commit
try handler ordering/guards/unmatched propagation
Result/Option projections
checked arithmetic reasons
return try
fallible property assignment
assert
panic reason/provenance
checked unreachable
cleanup on all applicable exits
```

§ 114(2) `try` must not absorb panic.

---

## § 115 Required test families: Iterator[T]

§ 115(1) Required tests include:

```text
user-defined concrete type explicitly conforms to Iterator[T]
generic Iterator[T] specialization
resolved Next() Option[T]
static Next target identity
None loop termination
Some value binding
by-value copy binding
shared borrow binding
mutable borrow binding
move-only yielded value rejection for plain copy iteration where applicable
iterator state destruction
iterator effect propagation
structural mutation conflict
no naming-convention iterator discovery
no mandatory runtime dispatch solely for Iterator[T]
compiler-known collection specialization remains semantically equivalent
```

§ 115(2) Compiler and LSP must resolve the same Iterator conformance/Next target.

---

## § 116 Required test families: collections/shaped

§ 116(1) Required tests include:

```text
collection kind/element metadata
backing invalidation operation
collection iteration dependencies
shaped rank/shape/stride/layout
scalar versus range indexing
tensor_view borrowing
broadcast/materialization effects
shape check preservation/elimination
```

---

## § 117 Required test families: concurrency

§ 117(1) Required tests include:

```text
task spawn/await/join
fallible spawn
owned/borrowed transfer
thread creation/join
task migration/thread-local affinity
channel ownership commit/failure
mutex guard capability
atomic operation identity/order
cancellation versus completion
structured concurrency cleanup
result dependency transfer
```

---

## § 118 Required test families: platform/unsafe

§ 118(1) Required tests include:

```text
FFI ABI/ownership/retention
unsafe provenance
RawPtr volatile operations
fixed-address operations
hardware-register plans
interrupt identity/root/generated wrapper
ISR effect guarantees
inline assembly semantic contract bridge
unsupported target rejection
```

---

## § 119 Required test families: lowering

§ 119(1) Required tests include:

```text
Semantic IR verification before Sec MLIR
semantic type identity survives until allowed lowering
no hidden allocation
no ownership inference in lowering
no iterator rediscovery in lowering
Arena domain/state/epoch preserved
panic versus Result preserved
reference/handle failure semantics preserved
hardware transaction semantics preserved
backend metadata no stronger than Sec proof
```

---

## § 120 Completion criteria

§ 120(1) Core Semantic IR support is complete when every Sec 0.1 source construct that survives semantic analysis has one canonical representation or resolved semantic plan consumable by lowering.

§ 120(2) Ownership/memory support is complete when Place identity, availability, copy/move, cleanup, references, storage, allocation, Arena, and destruction are explicit/proven through the IR.

§ 120(3) Error/panic support is complete when Result/Option/try/checked operations/assert/panic/cleanup remain distinct and verified.

§ 120(4) Iterator support is complete when `Iterator[T]` conformance and `Next() Option[T]` resolution are represented canonically without runtime-dispatch or closed-whitelist assumptions.

§ 120(5) Collections/shaped support is complete when their semantic descriptors, storage/view relationships, effects, and checks survive to appropriate lowering.

§ 120(6) Concurrency support is complete when task/thread/channel/synchronization/transfer/completion semantics are represented explicitly.

§ 120(7) Platform support is complete when FFI, volatile, hardware, interrupts, target operations, and inline-assembly contracts retain exact semantics.

§ 120(8) Verification/tooling support is complete when compiler, LSP, `sec analyse`, serialization, and Sec MLIR consume the same semantic identities/facts.

§ 120(9) Semantic IR must not be marked complete merely because one current Sec MLIR package can lower a subset of operations.

---

## § 121 Core summary

§ 121(1) Semantic IR is the canonical typed meaning of a validated Sec program before lowering.

§ 121(2) It is independent of source spelling and LLVM representation.

§ 121(3) Every ownership-changing, availability-changing, failure, panic, cleanup, allocation, and execution-boundary operation remains explicit until safely lowered or proven irrelevant.

§ 121(4) Canonical Place/storage/reference identities are shared with ownership, borrowing, lifetime, concurrency, and platform analyses.

§ 121(5) Amendments from earlier implementation packages are integrated into the relevant semantic sections rather than remaining separate historical layers.

§ 121(6) `Iterator[T]` is represented as a resolved compiler-known generic interface contract; `for` uses Sema-resolved `Next() Option[T]` semantics and does not rediscover iteration by naming convention.

§ 121(7) Statically resolved iterator conformance does not require runtime dynamic dispatch.

§ 121(8) Arena state/domain/epoch semantics remain explicit and distinct.

§ 121(9) Result/error propagation and panic remain separate control/failure mechanisms.

§ 121(10) Semantic IR preserves compiler-owned effects and verified guarantees.

§ 121(11) Tasks, threads, collections, shaped values, hardware, interrupts, and unsafe/FFI operations are first-class semantic concerns rather than lowering-only accidents.

§ 121(12) Sec MLIR implements Semantic IR meaning; it does not redefine the language.
