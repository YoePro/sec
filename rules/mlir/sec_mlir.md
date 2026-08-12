# Sec MLIR

## Status

Normative compiler architecture rulebook for Sec 0.1.

This rulebook defines the architecture, semantic boundaries, organization,
lowering model, verification philosophy, implementation order, versioning, and
completion criteria of the Sec MLIR layer.

It does not define the exact TableGen or C++ surface of the Sec dialect.

The exact dialect surface is defined separately by:

```text
rules/mlir/sec_mlir_dialect.md
```

That detailed specification must conform to this rulebook and must not introduce
or redefine Sec language semantics.

---

# Purpose

Sec uses MLIR as the structured lowering layer between validated Sec Semantic IR
and LLVM-oriented representation.

The compiler pipeline is:

```text
Sec source
    ↓
AST
    ↓
Sema
    ↓
Semantic IR
    ↓
Sec MLIR
    ↓
standard MLIR dialects
    ↓
LLVM dialect
    ↓
LLVM IR
```

Sec Semantic IR is the canonical representation of validated Sec semantics.

Sec MLIR is a lowering and transformation representation.

Sec MLIR exists to preserve all remaining Sec semantic obligations explicitly
while progressively converting validated Sec semantics into executable,
target-aware, lower-level representation.

Sec MLIR must not redefine the language.

# Normative role

This rulebook is normative for:

- the role of Sec MLIR in the compiler;
- the semantic boundary between Semantic IR and MLIR;
- the semantic boundary between Sec MLIR and standard MLIR;
- the semantic boundary between Sec MLIR and LLVM lowering;
- the organization of the Sec dialect;
- the categories of information that must remain explicit;
- the progressive lowering architecture;
- the conditions under which Sec-specific semantics may be erased;
- the verification architecture;
- the effect-preservation requirements;
- the relationship between layout and MLIR;
- the relationship between storage semantics and MLIR;
- the relationship between ABI lowering and high-level Sec MLIR;
- the implementation dependency order;
- the dialect-versioning policy;
- the testing and completion criteria for Sec MLIR 0.1.

This rulebook is not normative for the exact spelling or encoding of individual
Sec MLIR types, attributes, operations, interfaces, traits, builders, parser
forms, printers, or C++ classes.

Those details belong to `sec_mlir_dialect.md`.

# Relationship to other rulebooks

Sec MLIR consumes already-defined language and compiler semantics.

It does not own those semantics.

Relevant normative sources include, among others:

```text
semantic_ir.txt
storage.md
layout.md
ownership.md
borrowing.txt
reference_model.md
destruction.txt
effect_analysis.md
panic.md
runtime_checks.md
allocation.txt
arena.md
operators.md
collections.md
struct.txt
enums.txt
unions.txt
registers.txt
volatile.md
ffi.txt
```

The exact list may grow as Sec 0.1 is completed.

When this rulebook refers to ownership, storage, layout, panic, effects,
references, allocation, or another language/domain concept, the corresponding
domain rulebook defines the meaning.

Sec MLIR defines only how that meaning must be preserved and progressively
lowered.

# Normative authority

Normative authority flows in this order:

```text
language/domain rulebook
        ↓
semantic_ir.txt
        ↓
sec_mlir.md
        ↓
sec_mlir_dialect.md
        ↓
implementation
```

A lower layer must conform to all applicable higher layers.

If an implementation disagrees with `sec_mlir_dialect.md`, the implementation is
wrong.

If `sec_mlir_dialect.md` disagrees with this rulebook, the detailed dialect
specification is wrong.

If this rulebook disagrees with the relevant language/domain rulebook, this
rulebook is wrong.

If valid Semantic IR and the initial Sec MLIR representation disagree about
program semantics, that is a compiler bug.

# Design goals

Sec MLIR must:

- preserve validated Sec behavior;
- preserve semantic obligations until they are discharged;
- provide a verifiable transformation boundary;
- remain sufficiently high-level for ownership, lifetime, storage, reference,
  failure, and aggregate transformations;
- reuse standard MLIR dialects when they preserve the remaining semantics
  exactly;
- avoid duplicating standard MLIR merely to keep Sec source syntax visible;
- consume target and layout information without redefining it;
- remain independent of final callable ABI representation until ABI lowering;
- support progressive and partial lowering;
- support analysis and canonicalization without permitting semantic invention;
- expose effects sufficiently for safe optimization;
- preserve source locations and useful semantic provenance;
- make compiler invariant violations detectable;
- allow intermediate stages to be dumped and verified;
- support a migration from legacy direct lowering without making bypass paths
  permanent architecture;
- keep higher Sec compiler semantics independent of the C++ details required by
  MLIR infrastructure.

# Non-goals

Sec MLIR is not:

- a second Sec language definition;
- a representation of Sec source syntax;
- an alternate type checker;
- an alternate ownership checker;
- an alternate borrow checker;
- an alternate storage model;
- an alternate layout engine;
- an alternate ABI specification;
- a complete duplicate of standard MLIR;
- a virtual CPU instruction set;
- a permanent direct-to-LLVM code generator;
- a stable external IR ABI in Sec 0.1.

The detailed dialect specification must not use representation design as a way
to change language semantics.

# Canonical semantic boundary

Before entering MLIR lowering, Semantic IR verification must have succeeded.

All source-level semantic decisions required by the language must already have
been made.

This includes, as applicable:

- name resolution;
- type resolution;
- overload and method resolution;
- ownership validation;
- move/copy classification;
- borrow validation;
- reference escape validation;
- unsafe-context validation;
- control-flow validity;
- exhaustiveness validation;
- generic instantiation strategy;
- required concrete layouts for code generation;
- target restrictions;
- failure policy;
- required runtime-check semantics;
- explicit conversions;
- unit compatibility and unit conversions;
- function and call resolution.

MLIR lowering must not accept invalid Semantic IR as recoverable source input.

A failure of initial Sec MLIR construction from valid Semantic IR is a compiler
error.

# Semantic preservation until discharge

The central Sec MLIR rule is:

> A Sec semantic property may be erased only after its obligations have been
> proven statically, represented explicitly at a lower level, or materialized as
> required runtime behavior.

A lowering pass must never erase semantic information merely because a lower
representation is capable of holding the resulting bits.

Examples follow.

A checked array access may become ordinary address calculation only after:

- bounds safety has been proven; or
- the required bounds check has been materialized.

A safe reference may become an ordinary address-like value only after all
reference-specific validity requirements unavailable to the lower pointer form
have been:

- proven statically; or
- represented by runtime validation/protection; or
- transferred to a lower representation that provides the same guarantee.

A move may become an SSA transfer, load, or memory operation only after source
invalidation and cleanup responsibility are fixed.

A storage generation dependency may disappear only after stale-access safety is
proven or the required runtime generation mechanism has been materialized.

A union may lose high-level variant identity only after its tag/payload
representation and all active-variant obligations have been fixed.

A panic-capable operation may lose its high-level panic semantics only after its
panic behavior has been represented correctly at a lower level.

# Obligation model

Every remaining Sec safety or behavior requirement is conceptually a semantic
obligation.

Examples include:

```text
index must be within bounds
reference generation must still match
destination must be initialized before read
value must satisfy its contract
allocation failure must follow the selected policy
storage must not be reclaimed while protected
active union payload must match the selected variant
destruction must occur exactly as required
```

An obligation may leave the high-level Sec representation only through one of
these outcomes:

```text
StaticallyDischarged
RuntimeMaterialized
TransferredToLowerLevel
```

## Statically discharged

The compiler proves the obligation.

No runtime mechanism remains necessary for that obligation.

## Runtime materialized

The compiler creates the required runtime behavior.

Examples include:

```text
bounds check
generation validation
panic branch
Result error branch
lock or guard mechanism
allocation failure path
```

Once the runtime mechanism faithfully implements the obligation, the
corresponding high-level obligation may be removed.

## Transferred to lower level

A lower dialect, operation, type, or runtime mechanism may carry the same
guarantee.

Transfer is valid only when the lower representation provides the complete
remaining guarantee.

A lowering pass must not claim that an obligation was transferred merely because
the lower representation is convenient.

# No undocumented semantic convention

No lowering pass may rely on undocumented semantic information encoded only by
construction convention.

A later pass must not rely on assumptions such as:

```text
"this pointer happens to be pinned"
"this value is unique because this builder only creates unique values"
"this call cannot panic because the current producer never emits panic here"
"this address is MMIO because of where it came from"
```

If a later transformation depends on a semantic property, that property must be
available through an explicit and defined mechanism such as:

- a type;
- an attribute;
- an operation;
- an SSA dependency;
- a region/control-flow relationship;
- an operation interface;
- a defined compiler analysis.

# One Sec dialect

Sec 0.1 uses one Sec-specific MLIR dialect:

```text
sec
```

The Sec dialect may be split into multiple TableGen files, C++ source files,
operation families, verifier modules, transforms, and conversion libraries.

Those implementation divisions do not create separate semantic dialects.

Sec 0.1 does not define separate dialect namespaces such as:

```text
sec_storage
sec_ownership
sec_strings
sec_registers
```

A future split requires an explicit architectural change.

# Mixed-dialect IR

A high-level Sec MLIR module is not required to consist only of `sec.*`
operations or Sec-specific types.

Mixed-dialect IR is intentional.

Suitable standard dialects may coexist with the Sec dialect, including where
appropriate:

```text
builtin
func
arith
cf
scf
```

and other standard dialects selected by the compiler architecture.

Sec-specific representation is required only where direct use of standard MLIR
would:

- lose Sec semantics;
- obscure a remaining semantic obligation;
- prevent required verification;
- erase information prematurely;
- make a later transformation depend on reconstruction of source-language
  intent.

Ordinary standard MLIR must be preferred where it represents the remaining
semantics exactly.

Sec must not introduce equivalents such as `sec.i32`, `sec.add`, `sec.branch`,
or `sec.constant` merely because the operation originated in Sec.

# Initial Sec MLIR

The initial Sec MLIR representation should be structurally close to Semantic IR.

Semantic IR import should be mostly mechanical with respect to semantic meaning.

The initial representation must preserve enough structure that every remaining
semantic obligation can be identified and verified without reconstructing
source-language intent from low-level instructions.

The compiler must not translate Semantic IR immediately into near-LLVM form and
then attempt to rediscover ownership, lifetime, storage, reference, failure, or
aggregate semantics from low-level operations.

# CompilationPlan and target awareness

Sec MLIR begins after the relevant `CompilationPlan` has been selected.

The compiler must already know, as applicable:

```text
target
architecture
target profile
data layout
pointer width
concrete generic instantiations required for code generation
ResolvedLayout information
```

Sec MLIR is therefore target-aware and layout-aware.

It is not target-unknown IR.

However, high-level Sec MLIR remains independent of final callable ABI
representation until ABI lowering.

Target awareness does not imply early ABI lowering.

# ABI independence

A high-level Sec function retains its semantic function shape until ABI lowering
requires a concrete calling representation.

For example, a semantic function such as:

```sec
fn Process(value: LargeStruct) Result[int, Error]
```

must not be forced during initial Sec MLIR import into a platform-specific form
such as:

```text
hidden sret pointer
platform register pair
stack-only argument
platform-specific Result convention
```

unless that representation is already required by a defined earlier boundary.

High-level Sec MLIR must be capable of reaching different target ABI lowerings
without embedding one target ABI as its general function model.

ABI rules belong to `abi.md` and relevant FFI/target rulebooks.

# Layout model

Sec MLIR consumes resolved layout.

It does not define Sec layout.

The canonical layout engine defined by `layout.md` produces the target-aware
layout facts required by lowering.

Sec MLIR may retain a high-level semantic type while its physical layout is
already known.

For example, a tagged union may remain a high-level Sec union in MLIR while the
compiler already knows its resolved:

```text
size
alignment
tag layout
payload layout
padding
```

Representation lowering later consumes that layout information.

No MLIR lowering pass may invent a competing Sec layout algorithm.

No backend-specific convenience layout may override `ResolvedLayout`.

# Information model

Sec MLIR distinguishes four categories of compiler information:

```text
Types
Attributes
Operations
Analysis state
```

These categories are not interchangeable.

## Types

A type answers:

> What kind of runtime or semantic value is this at the current lowering level?

A Sec-specific MLIR type is appropriate while type-level Sec semantics must
remain explicit.

Possible semantic categories include:

```text
nominal values
safe references
raw pointers where Sec-specific provenance remains relevant
Result
Option
tagged unions
string
slice or descriptor values
register values
```

The exact type set belongs to `sec_mlir_dialect.md`.

Sec-specific types must be sparse.

Standard MLIR types must be used when they exactly represent the remaining
semantics.

Type identity and physical machine representation may be lowered independently.

A named Sec type may retain nominal identity while already having a known
physical representation.

Once no remaining semantic transformation depends on the nominal identity, that
identity may be erased from runtime representation while source/debug identity
may remain as metadata.

# Unit erasure

Units are compile-time semantic constraints.

Unit identity is not required to survive into Sec MLIR after all:

- dimensional compatibility;
- relation validation;
- scale/conversion selection;
- overflow behavior;
- rounding behavior;

have been resolved.

Any runtime conversion required by a unit operation must already exist as
explicit ordinary computation before unit identity is erased.

For example, if conversion between two compatible units requires runtime
multiplication or division, the required computation must remain.

The unit label itself need not remain as runtime or transformation semantics.

Unit information may survive as debug or source-provenance metadata.

Sec MLIR does not require dedicated runtime types such as:

```text
!sec.meter
!sec.foot
!sec.kilogram
```

merely to preserve already-resolved unit checking.

# Attributes

Attributes describe immutable semantic or representation facts associated with
an entity or operation.

Candidate categories include:

```text
StorageOrigin
BackingRelation
ReclamationAuthority
AddressStability
MemorySpace identity

resolved layout identity
alignment requirements
field identity
variant identity

failure policy
allocation policy
calling-convention intent before ABI lowering

nominal type identity
contract identity
source semantic flags
```

The exact attribute design belongs to `sec_mlir_dialect.md`.

Stable semantic facts should normally be represented declaratively rather than
encoded indirectly through opaque operation sequences.

# Runtime and control-flow-dependent state

Runtime or path-dependent state must not be represented as immutable compile-time
facts.

Examples include:

```text
current storage generation
current object lifetime
current occupant of a reusable slot
active runtime pin state
active runtime protection
Result success/error state
```

Such state must be represented through appropriate combinations of:

- SSA values;
- operations;
- control flow;
- runtime storage;
- lower-level synchronization or protection mechanisms.

A compile-time dependency such as:

```text
this reference depends on invalidation domain D
```

may be represented statically.

A runtime generation value must not be confused with such a static dependency.

# Operations

Operations represent semantic actions or transitions that remain observable or
relevant to lowering.

A semantic event must remain explicit until its obligations have been discharged
or materialized.

Operation families are defined architecturally below.

The exact operation set belongs to `sec_mlir_dialect.md`.

# Analysis state

Compiler analyses may know facts such as:

```text
reference proven valid here
allocation proven impossible
call proven non-blocking
value proven unique
generation check proven redundant
storage proven non-escaping
branch proven unreachable
stack usage estimate
call graph properties
```

Analysis results are not automatically persistent Sec MLIR metadata.

Analysis facts belong to compiler analyses unless a later transformation
requires them as a durable transformation contract.

This rule prevents stale analysis metadata from surviving transformations after
the proof conditions have changed.

# Sec semantic regions and MLIR regions

Sec semantic regions and MLIR regions are different concepts.

The term:

```text
Sec semantic region
```

means the compiler-internal lifetime/storage dependency concept defined by the
Sec storage/lifetime model.

The term:

```text
MLIR region
```

means an MLIR IR container containing blocks and operations.

A Sec semantic region need not map to one MLIR region.

An MLIR region does not automatically define a Sec lifetime or storage region.

The detailed dialect specification must not conflate these concepts.

# Storage-domain identity, address identity, and generation

These are separate concepts:

```text
storage-domain identity
address identity
generation
```

A storage domain is not a pointer.

Different storage-domain incarnations may use the same numeric address at
different times.

Sec MLIR must preserve the distinction for as long as any validity or
reclamation semantics depend on it.

Lowering may erase the distinction only after the relevant safety obligations
have been discharged or materialized.

# Safe references

Safe Sec references must remain semantically qualified until every
reference-specific validity obligation unavailable to an ordinary machine
pointer has been discharged or materialized.

A safe reference may conceptually depend on facts such as:

```text
address
object lifetime
storage domain
epoch or generation
borrow rights
mutability
pin or protection state
memory space
```

The exact representation belongs to `sec_mlir_dialect.md`.

Safe references must not be lowered prematurely to unqualified machine pointers.

Reference derivation must preserve every validity dependency required by the
derived reference unless that dependency has already been discharged.

For example:

```text
ref container
    ↓
ref container.field
```

must preserve the lifetime/storage dependencies required to keep the derived
field reference valid.

`RawPtr[T]` is distinct from a safe reference and follows the explicit unsafe
raw-pointer semantics defined elsewhere.

# Ownership independence

The following concepts remain independent:

```text
owns value
owns backing storage
may reclaim backing storage
has exclusive reference
contains pointer
```

Sec MLIR must not use pointer presence as a proxy for ownership.

Sec MLIR must not use value ownership as a proxy for backing-storage ownership.

Sec MLIR must not use backing-storage ownership as a proxy for reclamation
authority.

These distinctions follow the ownership and storage rulebooks.

# Dialect organization

The Sec dialect is organized internally into semantic families.

The recommended architecture is:

```text
Core
Types
Values
Aggregates
Ownership
Lifetime
Storage
References
Checks
ErrorFlow
Allocation
Strings
Concurrency
Hardware
Functions
Platform
```

These are organizational and lowering boundaries.

They are not separate MLIR dialects and do not define new language semantics.

Not every family must contain a custom operation for every Sec feature.

# Core family

Core contains shared infrastructure such as:

```text
dialect registration
common attributes
common interfaces
common traits
common verifier utilities
type identity support
layout references
effect representation
semantic provenance
```

Core must not become a miscellaneous operation collection.

# Values and nominal semantics

This family represents value semantics that must remain visible after source
syntax has disappeared.

Possible concerns include:

```text
named types
distinct types
constrained types
explicit representation conversions
compiler-known value conversions
```

Nominal semantics remain explicit only while some remaining operation,
verification rule, or lowering decision depends on them.

Once those dependencies are complete, the physical representation may lower to
a standard MLIR type.

# Aggregate family

The aggregate family covers high-level value representations such as:

```text
struct
fixed array
slice descriptor
enum
union
Option
Result
```

Possible operations include semantic construction, extraction, update, variant
selection, and payload extraction.

Standard MLIR aggregate operations should be reused whenever they preserve all
remaining Sec semantics.

Sec-specific aggregate operations remain necessary where the compiler still
depends on semantics such as:

```text
active variant
object lifetime
ownership transfer
partial initialization
Result/error meaning
```

# Ownership family

Ownership operations represent ownership actions that must not be reduced
prematurely to ordinary loads, stores, or copies.

Conceptual actions include:

```text
move
copy
ownership transfer
consume
```

A semantic move is not equivalent to `memcpy`.

A semantic copy is not automatically equivalent to `memcpy`.

Ownership lowering must preserve:

- source validity;
- destination validity;
- Copy eligibility;
- ownership transfer;
- partial-move state;
- cleanup responsibility.

# Lifetime family

Object lifetime is separate from ownership and storage reclamation.

Conceptual lifetime actions include:

```text
construct object
begin object lifetime
end object lifetime
destroy object
replace object
```

The detailed dialect may combine or split concrete operations where equivalent,
but it must preserve the semantic distinctions required by the domain rules.

In particular:

```text
destruction
object lifetime termination
storage reclamation
```

must not be collapsed prematurely.

# Storage family

The storage family consumes the storage semantics defined by `storage.md`.

It must be capable of representing, directly or equivalently, remaining
semantic actions concerning:

```text
domain establishment
epoch advancement
domain end

backing binding
backing rebinding
backing relocation

pinning
reclamation protection
generation validation
```

The storage family must not:

- choose hidden allocation;
- invent a storage origin;
- infer a new lifetime;
- redefine reclamation authority;
- redefine layout.

It represents already-decided semantics.

# Reference family

The reference family preserves safe-reference semantics while reference
obligations remain active.

Possible conceptual actions include:

```text
borrow
mutable borrow
reborrow
reference derivation
field reference
index reference
reference validation
reference narrowing
end internal borrow dependency
```

The detailed dialect decides which of these require explicit operations and
which are represented through types, attributes, SSA dependencies, or
interfaces.

# Check family

Checks use a common architectural model.

Check categories include, as applicable:

```text
bounds
overflow
contract
generation
capacity
alignment
range
variant
```

A check conceptually has:

```text
condition
failure policy
source provenance
effects
```

Failure policy may lower to:

```text
panic
Result/error
proven unreachable
platform trap
```

when allowed by the relevant language rule.

Feature-specific checks should not be implemented as unrelated ad-hoc compiler
mechanisms when the common check model can express them.

# Error-flow family

Result/error flow and panic flow remain distinct until explicitly lowered.

High-level error-flow semantics may include:

```text
success
error
propagate
handle
panic
unreachable after panic
```

For example, a fallible allocation performed under `try` has explicit error
flow.

The same allocation under a panic-accepting policy has panic behavior instead.

Checked integer arithmetic preserves a typed failure reason in Sec MLIR until
the failure policy has been made explicit. Schema 5 carries `(result, failed,
reason)` from checked operations and passes `reason` to the dedicated failure
successor. An ordinary path terminates with `sec.fail.arithmetic(reason)`; a
naked arithmetic `try` path maps the reason to exact core `ArithmeticError`,
constructs semantic `Result.err`, and returns it. No runtime call or physical
Result/error layout is implied by these high-level operations.

One must not be silently converted into the other.

# Allocation family

Allocation and storage identity are related but distinct.

The allocation family may represent:

```text
allocation
fallible allocation
deallocation through authority
arena allocation
explicit allocation context
```

Allocation operations must preserve:

- selected allocation context;
- failure policy;
- requested resolved layout;
- resulting ownership semantics.

No Sec MLIR operation or lowering pass may introduce hidden dynamic allocation
where source/compiler semantics do not permit it.

# String family

String values and string operations may remain high-level while doing so
preserves semantic clarity or enables safe optimization.

The family may include:

```text
string construction
StringConcatPlan
formatted segment
string comparison where Sec semantics remain relevant
string conversion
string storage interaction
```

A maximal string concatenation/interpolation expression may remain one semantic
concat plan.

It must not be forced prematurely into a chain of independently allocating
binary helper calls.

Built-in formatting may be fused into the plan where the language rules allow
it.

User-defined operations retain their declared effects and evaluation semantics.

# Concurrency and reclamation-protection family

Sec MLIR does not need to model all concurrency as a new high-level concurrency
language.

However, when safe transformation depends on a protection mechanism, that
dependency must remain explicit.

Examples include:

```text
lock guard
pin
hazard protection
epoch protection
atomic slot protocol
compiler-known reclamation protection
```

Protection establishes an IR dependency that transformations must preserve.

A load, store, reference use, or generation validation that is safe only within
a guard must not be moved outside the guard by optimization.

Generation validation alone does not make concurrent invalidation safe.

The reclamation rules remain those defined by `storage.md`.

# Hardware family

Hardware semantics must remain explicit while ordinary MLIR memory operations
would be insufficient.

Possible concerns include:

```text
register read/write
MMIO access
volatile access
barrier
fixed-address access
special-width access
target-known hardware operation
```

Pure bit manipulation that no longer carries hardware access semantics may use
ordinary standard arithmetic/bitwise operations.

Hardware lowering must preserve:

- access width;
- volatility;
- ordering;
- barriers;
- fixed-address semantics;
- memory spaces;
- observable side effects.

# Functions

The architecture does not require a custom `sec.func` operation for every Sec
function.

Standard MLIR function operations should be preferred when they can carry all
remaining semantics through:

- function types;
- attributes;
- operation interfaces;
- surrounding IR metadata.

A custom Sec function operation should be introduced only if the detailed
dialect design demonstrates a concrete semantic requirement that standard
function operations cannot represent adequately.

The same principle applies to calls.

High-level function representation may need to retain facts such as:

```text
effect contract
panic contract
allocation contract
source/generic identity
pre-ABI calling intent
```

but those facts need not imply a custom function operation.

# Control flow

Sec MLIR should reuse standard structured and control-flow dialects where their
semantics are sufficient.

The architecture does not require custom forms such as:

```text
sec.if
sec.loop
sec.branch
```

without a concrete semantic need.

Cleanup-, lifetime-, ownership-, and error-sensitive control flow may require
Sec-specific high-level representation until the relevant obligations are
fixed.

Once those obligations are explicit and preserved, ordinary `scf` or `cf`
representation may be used where valid.

# Result and Option

`Result` and `Option` may remain high-level semantic values after their physical
layout is known.

They need not immediately become explicit tag/payload machine structures.

Retaining high-level semantics may enable:

- propagation folding;
- dead error-path elimination;
- high-level variant reasoning;
- later ABI optimization;
- clearer verification.

The concrete representation is determined by resolved layout and representation
lowering.

# Tagged unions

Tagged unions may retain high-level variant identity after their concrete layout
is known.

Sec 0.1 uses the union representation defined by `layout.md`.

High-level Sec MLIR may therefore know both:

- the semantic active variant;
- the already-resolved physical tag/payload layout.

Representation lowering consumes the physical layout later.

High-level union semantics must not be reduced prematurely to an integer tag and
raw byte buffer if later verification still depends on variant identity.

# Effects

Sec effect semantics are defined by `effect_analysis.md`.

Sec MLIR must expose enough effect information to prevent invalid
transformations.

Relevant effects include, as applicable:

```text
Read
Write
Allocate
Deallocate
Panic
ReturnError
Block
Suspend
Spawn
Invalidate
Reclaim
PerformIO
AccessVolatile
Synchronize
```

The exact mapping to MLIR effect interfaces, custom interfaces, attributes, or
analysis infrastructure belongs to `sec_mlir_dialect.md`.

An operation must not be considered pure merely because it lacks an ordinary
memory write.

Panic, error propagation, allocation, destruction, volatile access, MMIO,
synchronization, invalidation, reclamation, blocking, and IO may all be
observable behavior.

# Interfaces

Cross-cutting semantic queries should normally be expressed through MLIR
operation interfaces or equivalent defined abstractions rather than giant
operation-name switches.

Conceptual interface families may include:

```text
SecEffectInterface
SecOwnershipInterface
SecLifetimeInterface
SecStorageInterface
SecReferenceInterface
SecCheckInterface
SecLoweringInterface
SecLayoutConsumerInterface
```

These names are illustrative.

The detailed dialect specification defines the actual interface set and names.

A pass should be able to ask semantic questions such as:

```text
Does this operation consume an owned value?
Can this operation invalidate a storage domain?
Can this operation panic?
Does this result derive reference dependencies?
Does this operation require resolved layout?
```

without encoding knowledge of every concrete operation.

# Traits

Traits are appropriate for local, structural properties.

Possible conceptual properties include:

```text
NoPanic
NoAllocation
ConsumesOperand
ProducesOwnedValue
RequiresResolvedLayout
Terminator
PureWhenOperandsPure
```

The exact trait set belongs to `sec_mlir_dialect.md`.

Traits must not substitute for control-flow-sensitive analysis.

A complex lifetime statement such as:

```text
reference remains valid until a particular dynamic protection ends
```

must not be reduced to a simple static trait if that would lose semantics.

# Source locations and semantic provenance

Source locations must be retained through lowering while useful for:

- diagnostics;
- runtime trap attribution;
- debug information;
- compiler invariant reporting;
- optimization remarks;
- stack/call analysis;
- mentor diagnostics.

Compiler-generated operations should retain derived source provenance where a
meaningful source origin exists.

Semantic provenance may survive after runtime semantics are lowered when useful
for diagnostics or debugging.

Debug metadata may retain source type names, source variable names, field names,
variant names, and related information after the corresponding runtime
semantics have been fully lowered.

Debug metadata must not be used as a substitute for semantic IR state required
for correctness.

# Verification architecture

Sec MLIR verification primarily protects compiler invariants.

Frontend semantic validation has already occurred.

Invalid Sec MLIR usually indicates a compiler bug, invalid serialized compiler
artifact, invalid transformation, or malformed manually authored test IR.

Verification is divided into three levels.

## Local operation verification

Local verification includes properties such as:

```text
operand types
result types
required attributes
layout consistency
valid enum/union variant identity
valid memory-space use
locally valid ownership form
locally valid effect declarations
```

## Region/function verification

Function- or region-level verification includes relations such as:

```text
ownership usage
cleanup completeness
object lifetime consistency
borrow/reference dependencies
panic/effect contracts
path-sensitive initialization
path-sensitive move state
```

## Module/whole-program verification

Module or whole-program verification includes properties such as:

```text
cross-function contracts
symbol resolution
target consistency
ABI readiness
dialect version compatibility
remaining forbidden Sec semantics before final lowering
```

The exact implementation split may differ, but all required invariant classes
must be covered.

# Verification after lowering stages

Each semantically significant lowering stage must have a defined verifiable IR
contract.

Conceptually:

```text
import
    ↓ verify

normalization
    ↓ verify

ownership/lifetime/storage lowering
    ↓ verify

runtime lowering
    ↓ verify

representation lowering
    ↓ verify

platform/ABI lowering
    ↓ verify
```

A release compiler need not run every expensive verifier after every pass.

The architecture nevertheless requires that each defined stage can be verified
during development and testing.

# Folding, canonicalization, and lowering

These are distinct transformations.

## Folding

Folding computes a semantically known result statically.

Examples include:

```text
constant arithmetic
constant string concatenation
SizeOf
AlignOf
StrideOf
known layout query
known enum conversion
```

Folding must follow Sec semantics.

## Canonicalization

Canonicalization replaces IR with a simpler semantically equivalent form.

Examples may include:

- merging adjacent constant string-concat segments;
- eliminating redundant semantic wrappers after their obligations are complete;
- normalizing equivalent check forms;
- simplifying already-proven branches.

Canonicalization must not invent new semantics.

## Lowering

Lowering changes abstraction level after the relevant obligations have been
handled.

Lowering may replace high-level Sec operations with standard MLIR operations,
runtime mechanisms, platform operations, ABI representation, or LLVM-oriented
representation.

# Canonicalization restrictions

Canonicalization must not:

- change failure policy;
- create hidden allocation;
- change storage origin;
- change backing relation;
- change reclamation authority;
- change ownership;
- assume stronger reference validity;
- weaken lifetime rules;
- weaken generation requirements;
- change observable evaluation order;
- reorder observable destruction;
- move protected accesses outside required guards;
- erase observable effects.

For example, a panic-capable string concatenation must not be canonicalized into
a fallible `Result`-producing concatenation.

Likewise, a fallible concatenation under `try` must not be canonicalized into a
panic-only operation.

# Effect-aware optimization

Dead-code elimination and reordering must respect all observable Sec effects.

An operation may be removed as dead only when all of its observable effects are
proven irrelevant.

Observable effects include more than memory mutation.

They include, as applicable:

```text
panic
error propagation
allocation
deallocation
destruction
volatile access
MMIO
synchronization
domain invalidation
reclamation
blocking
IO
```

Correct code must never depend on an optimizer being present.

Optimization may improve the implementation of already-correct semantics.

It must not replace semantic validation.

# Progressive and partial lowering

Sec lowering is progressive.

A pass should lower only the semantic area it owns.

The compiler architecture must not depend on one monolithic:

```text
LowerSecToLLVM
```

step that collapses all Sec semantics at once.

Different Sec operations may exist at different abstraction levels in the same
module while this is legal for the current stage.

For example, one pass may eliminate a proven-safe bounds check while storage
generation validation remains high-level.

Partial lowering is intentional.

# Dialect conversion legality

Each lowering stage may define operations and types as:

```text
legal
dynamically legal
illegal
```

The stage contract must state which Sec semantics are permitted to remain.

For example:

- after a representation-lowering stage, a high-level union-construction
  operation may be illegal;
- after complete runtime-check lowering, a high-level bounds-check operation may
  be illegal;
- before LLVM translation, any remaining runtime-relevant Sec-only semantic
  operation is illegal.

This model should use MLIR dialect-conversion mechanisms where practical.

# Lowering architecture

The canonical logical stages are:

```text
Stage 0 — Verified Semantic IR
        ↓
Stage 1 — High-level Sec MLIR
        ↓
Stage 2 — Semantic normalization
        ↓
Stage 3 — Lifetime / ownership / storage lowering
        ↓
Stage 4 — Runtime semantics lowering
        ↓
Stage 5 — Representation lowering
        ↓
Stage 6 — Platform lowering
        ↓
Stage 7 — ABI lowering
        ↓
Stage 8 — Standard MLIR / LLVM dialect
        ↓
Stage 9 — LLVM IR
```

These are logical stages.

They do not require exactly one compiler pass each.

The compiler may split or combine implementation passes as long as the semantic
dependencies and stage contracts remain valid.

# Stage 0 — Verified Semantic IR

Semantic IR has passed all required Semantic IR verification.

No unresolved source-level semantic error may enter normal MLIR lowering.

All Sec language/domain semantics required for lowering are already known.

# Stage 1 — High-level Sec MLIR

The initial Sec MLIR is imported from Semantic IR with minimal semantic
reinterpretation.

It may retain high-level concepts such as:

```text
ownership
move/copy
object lifetime
safe references
storage domains
Result
Option
tagged unions
StringConcatPlan
register operations
checks
```

The initial representation must be verifiable.

# Stage 2 — Semantic normalization

This stage establishes canonical high-level Sec forms.

Possible transformations include:

- normalization of equivalent operations;
- flattening of string concat plans;
- normalization of checked operations;
- exposure of cleanup paths;
- preparation of effect information;
- lowering or expansion of compiler-known operations where defined.

No semantic obligation may be silently dropped.

# Stage 3 — Lifetime, ownership, and storage lowering

This stage resolves high-level semantics concerning:

```text
move
copy
destruction
cleanup
object lifetime
storage domains
epoch behavior
pinning
reclamation protection
```

as far as possible.

Object lifetime semantics must be explicit before reclamation lowering can be
considered complete.

Ownership metadata may be erased only after cleanup and transfer responsibility
are fixed.

Generation semantics may be erased only after validation/protection is proven or
materialized.

# Stage 4 — Runtime semantics lowering

This stage materializes runtime behavior required by remaining obligations.

Examples include:

```text
generation checks
bounds checks
contract checks
allocation failure paths
panic paths
Result propagation machinery
runtime helper calls where permitted
guard/protection machinery
```

After an obligation is faithfully materialized, its redundant high-level form
may be removed.

# Stage 5 — Representation lowering

This stage consumes `ResolvedLayout` and materializes lower-level
representations.

Examples include:

```text
struct representation
union tag and payload
Option representation
Result representation
descriptor representation
arrays
strings
safe-reference runtime representation
aggregate storage
storage-domain runtime state
```

Representation lowering must use the canonical layout model.

It must not recompute or alter Sec layout independently.

# Stage 6 — Platform lowering

This stage resolves target/platform semantics such as:

```text
register operations
MMIO
volatile access
target-specific operations
fixed-address operations
memory-space-specific operations
syscall-facing details where applicable
```

Platform lowering must preserve the target rules and effects defined elsewhere.

# Stage 7 — ABI lowering

ABI lowering resolves:

```text
calling convention
argument classification
return classification
sret or equivalent mechanisms
register versus stack passing
foreign ABI representation
symbol/call representation details
```

High-level Sec MLIR must not hard-code one final ABI before this stage except
where a defined extern/foreign boundary explicitly requires earlier ABI
knowledge.

# Stage 8 — Standard MLIR / LLVM dialect

All remaining operations must be representable using dialects accepted by the
final LLVM translation path.

No runtime-relevant Sec semantic obligation may remain merely as undocumented
metadata.

# Stage 9 — LLVM IR

LLVM IR translation is the final lowering boundary.

Normal LLVM emission requires zero remaining Sec-only runtime semantics whose
obligations have not been discharged.

Any Sec-specific information intentionally retained for debug metadata or other
non-runtime purposes must not be required for runtime correctness.

A Sec operation that reaches LLVM translation without a defined valid lowering
is a compiler invariant failure.

# Pass-order dependencies

This rulebook defines semantic dependencies rather than one rigid global pass
sequence.

Required dependencies include:

```text
ownership metadata may be erased only after cleanup responsibility is fixed

generation semantics may be erased only after validation/protection has been
proven or materialized

union semantic identity may be erased only after tag/payload representation has
been fixed and active-variant obligations are resolved

layout-independent aggregate operations must be resolved before a lowering that
requires physical offsets

ABI lowering requires resolved physical representation

protected accesses must remain within their required protection scope until
that protection has been lowered to an equivalent mechanism
```

The optimizer and pass manager may reorder independent transformations when the
result remains valid under all applicable constraints.

# Implementation organization

The architecture recommends one implementation tree for the Sec dialect.

A possible organization is:

```text
include/sec/Dialect/Sec/
    SecDialect.td
    SecTypes.td
    SecAttributes.td
    SecInterfaces.td
    SecTraits.td

    SecAggregateOps.td
    SecOwnershipOps.td
    SecLifetimeOps.td
    SecStorageOps.td
    SecReferenceOps.td
    SecCheckOps.td
    SecErrorOps.td
    SecAllocationOps.td
    SecStringOps.td
    SecConcurrencyOps.td
    SecHardwareOps.td
```

and:

```text
lib/Dialect/Sec/
    SecDialect.cpp
    SecTypes.cpp
    SecAttributes.cpp
    SecInterfaces.cpp
    SecVerify.cpp

    Transforms/
    Conversion/
```

The exact file structure is not normative.

`sec_mlir_dialect.md` may refine it.

The architectural requirement is one coherent `sec` dialect with modular
implementation boundaries.

# Implementation order

Implementation proceeds according to semantic dependencies.

The compiler does not need to follow one exact commit order.

The following phases define the recommended dependency order.

## Phase 0 — Infrastructure

Implement:

```text
dialect registration
build integration
TableGen generation
dialect loading
parser/printer infrastructure
basic verifier infrastructure
dialect test harness
```

The milestone is the ability to construct, parse/print where supported, and
verify a minimal Sec dialect module.

## Phase 1 — Fundamental representation

Implement the shared representation needed by later features.

This includes, as required:

```text
common attributes
type identity
layout references
effect interfaces
source provenance
basic Sec-specific types
Semantic IR -> high-level Sec MLIR bridge
```

The milestone is that a simple validated Sec program can enter high-level Sec
MLIR without semantic loss and can be dumped and verified.

## Phase 2 — Aggregates and ordinary values

Implement the fundamental value/aggregate model, including as applicable:

```text
structs
fixed arrays
enums
tagged unions
Option
Result
string value representation
```

This phase consumes resolved layout information but need not immediately lower
all values to LLVM physical representation.

## Phase 3 — Ownership and object lifetime

Implement:

```text
move
copy
consume
construction
destruction
object lifetime end
cleanup paths
partial initialization
partial move
```

Object lifetime must be explicit before reclamation lowering is considered
complete.

## Phase 4 — References and storage

Implement:

```text
safe references
reference derivation
borrow dependencies

storage domains
epochs
runtime generations
backing relations
pinning
reclamation protection
```

The milestone is the ability to represent the complete safe-reference/storage
model without prematurely reducing safe references to raw machine pointers.

## Phase 5 — Checks and failure semantics

Implement the common check/failure model, including as applicable:

```text
bounds checks
overflow checks
contract checks
generation checks
capacity checks
alignment checks

panic flow
Result/error flow
try propagation
```

Checks should use shared architectural machinery rather than feature-specific
ad-hoc lowering paths.

## Phase 6 — Allocation, arenas, and strings

Implement:

```text
allocation contexts
allocation failure
arena allocation
arena reset/release
string allocation
StringConcatPlan
formatting segments
```

This phase depends on the storage and failure models being sufficiently
available.

## Phase 7 — Effect and optimizer integration

Expand effect modeling and optimizer integration until MLIR transformations can
correctly reason about operations that may:

```text
panic
allocate
deallocate
invalidate
reclaim
perform volatile access
synchronize
block
perform IO
```

Basic effect infrastructure may be introduced earlier.

This phase completes the broader optimizer-facing contract.

## Phase 8 — Hardware and platform

Implement target-aware Sec/platform representation for:

```text
registers
MMIO
volatile
fixed-address access
barriers
target-known operations
memory-space-sensitive operations
```

This phase remains separate from final ABI lowering.

## Phase 9 — Representation lowering

Systematically lower high-level values and runtime state to standard MLIR and
lower-level representation.

Correct high-level operation semantics and verification must exist before the
corresponding lowering is considered complete.

The implementation principle is:

```text
High-level operation implementation first.
Correct lowering second.
Optimization third.
```

## Phase 10 — ABI lowering

Implement ABI conversion after the relevant ABI rulebook and target-specific
contracts are available.

The Sec dialect architecture must remain capable of supporting multiple ABIs.

# Legacy lowering migration

The compiler may contain existing or temporary direct MLIR/LLVM lowering paths
during migration.

Such paths are migration mechanisms, not the target architecture.

No new Sec language feature should receive a permanent feature-specific bypass:

```text
Semantic IR -> handcrafted LLVM
```

merely because the corresponding Sec dialect operation has not yet been
implemented.

Temporary compatibility paths must be identifiable and removable.

The target architecture is:

```text
Semantic IR
    ↓
Sec MLIR
    ↓
progressive lowering
    ↓
LLVM dialect
    ↓
LLVM IR
```

# Dialect versioning

The Sec MLIR dialect has an internal version independent of the Sec language
version.

Sec language version `0.1` may be compiled through multiple Sec dialect
revisions while the compiler is being developed.

The dialect version identifies the semantics and representation expected by
serialized Sec MLIR.

Language-version compatibility does not imply serialized-IR compatibility.

# Serialized IR compatibility

Sec 0.1 does not guarantee that serialized Sec MLIR produced by one compiler
build can be consumed by another compiler build.

The following are not stable external contracts in Sec 0.1 unless separately
specified:

```text
textual Sec MLIR syntax
Sec MLIR bytecode
plugin-facing Sec MLIR ABI
cross-version external tooling compatibility
```

The compiler may later stabilize such formats.

No such stability is implied by this rulebook.

# Version mismatch

When serialized Sec MLIR is consumed, tooling must be able to detect incompatible
dialect versions when versioned semantics are required.

An incompatible dialect version must be rejected deterministically.

The compiler must not reinterpret unknown or changed operation semantics as an
older version on a best-effort basis.

The detailed dialect specification defines the concrete version encoding.

# Implementation status model

The detailed dialect specification should track implementation progress for
major operation/type families using states equivalent to:

```text
Specified
Implemented
Verified
LoweringImplemented
Tested
ProductionReady
```

These states are independent.

An operation is not production-ready merely because it can be parsed or
constructed.

# Testing strategy

Sec MLIR requires three primary test levels.

## Dialect unit tests

Dialect unit tests cover:

```text
types
attributes
operations
builders
parser/printer behavior where relevant
local verifiers
cross-operation verifiers
folding
canonicalization
conversion patterns
```

Safety-relevant operations must include intentionally invalid IR tests.

## Pipeline tests

Pipeline tests cover one or more lowering boundaries.

Intermediate forms must be verifiable.

Pipeline tests should verify semantic invariants rather than only checking that a
pass exits successfully.

## End-to-end tests

End-to-end tests cover:

```text
Sec source
    ↓
compiler pipeline
    ↓
native executable or target artifact
```

They must verify observable behavior.

Where relevant this includes:

- normal results;
- panic behavior;
- Result/error propagation;
- destruction order;
- allocation failure policy;
- reference validity behavior;
- string behavior;
- hardware-visible behavior on appropriate targets.

# Negative verifier tests

Negative verifier tests are first-class tests.

They deliberately construct IR that normal Sec frontend processing would never
produce.

Examples include:

```text
double move
destroy after move
invalid union payload
missing generation dependency
reclaim while protected or pinned
wrong layout attachment
illegal effect contract
invalid Result extraction
invalid reference derivation
```

These tests verify that the Sec dialect acts as a compiler correctness firewall.

# Golden IR tests

Golden textual IR tests should be used selectively.

They are appropriate for:

- canonical operation syntax;
- important canonical forms;
- stable representation contracts;
- critical lowering boundaries.

Tests should not become unnecessarily brittle when several equivalent MLIR forms
are valid.

Semantic property checks are preferred when exact line-for-line output is not
normative.

# Differential testing during migration

While an older lowering path and the new Sec MLIR path both support the same
feature, the project should use differential testing where practical.

Comparison may include:

```text
runtime result
panic/error behavior
layout
destruction
observable side effects
generated target behavior
```

Differential testing is a migration technique.

It is not a permanent architectural dependency on the old lowering path.

# Debug and IR dumps

Compiler tooling must support observing meaningful intermediate representations.

At minimum the architecture must permit dumps equivalent to:

```text
Semantic IR
initial Sec MLIR
normalized Sec MLIR
post-lifetime/ownership/storage lowering
post-runtime lowering
post-representation lowering
post-platform/ABI lowering
LLVM dialect
LLVM IR
```

Exact command-line flag names are not defined here.

Intermediate stages are legitimate compiler artifacts for development,
verification, testing, and diagnostics.

# Stopping at intermediate stages

The compiler architecture must permit compilation to stop after a selected
lowering stage.

Conceptually, tooling may provide capabilities equivalent to:

```text
emit high-level Sec MLIR
emit runtime-lowered Sec MLIR
emit LLVM dialect MLIR
emit LLVM IR
```

The exact CLI surface belongs elsewhere.

Each exposed stage must satisfy its defined verification contract.

# Sec MLIR 0.1 completion criteria

Sec MLIR 0.1 is complete when every Sec 0.1 semantic feature that requires code
generation satisfies all applicable requirements below:

1. It can be represented in high-level Sec MLIR without semantic loss.
2. Its remaining semantic obligations can be identified.
3. Its representation can be verified.
4. Its required lowering is defined.
5. It can be progressively lowered through the architecture.
6. It reaches LLVM translation without unresolved runtime-relevant Sec semantics.
7. It preserves failure behavior.
8. It preserves effect behavior.
9. It preserves ownership semantics.
10. It preserves object-lifetime semantics.
11. It preserves storage and reclamation semantics.
12. It preserves safe-reference semantics.
13. It preserves layout semantics.
14. It has positive dialect/pipeline tests where applicable.
15. It has negative verifier tests where applicable.
16. It has lowering tests.
17. It has end-to-end behavior tests.
18. It does not require a permanent feature-specific direct-to-LLVM bypass.

Completion is not defined as merely having TableGen definitions for every
planned operation.

# Bootstrapping boundary

MLIR dialect infrastructure is expected to require C++ and TableGen integration
because MLIR itself is implemented around those mechanisms.

That does not imply that higher Sec compiler semantics must be implemented in
C++.

The architectural boundary is:

```text
Sec compiler semantic logic
        ↓
defined internal bridge
        ↓
MLIR dialect implementation
```

Higher compiler layers should depend on semantic interfaces and the defined
bridge, not on incidental C++ implementation details.

This boundary allows progressively more compiler logic to be written in Sec
without requiring Sec to reimplement MLIR infrastructure.

# Relationship to sec_mlir_dialect.md

`sec_mlir_dialect.md` is the detailed implementation specification.

It defines the exact surface required to implement the dialect.

For each significant operation or type it may define items such as:

```text
Purpose
Syntax
Operands
Results
Attributes
Regions
Effects
Traits
Interfaces
Verification
Canonicalization
Folding
Lowering preconditions
Lowering result
Invalid IR examples
Valid IR examples
TableGen definition
C++ implementation notes
Tests
```

The detailed document should be mechanical and implementation-oriented.

It should not duplicate long explanations of why the underlying Sec semantics
exist.

Those explanations belong to the relevant language/domain rulebooks.

# Representation freedom of sec_mlir_dialect.md

The detailed dialect specification may choose local MLIR representation details
within this architecture.

For example, it may determine whether a particular storage-generation
dependency is encoded through:

```text
an SSA operand
a dedicated domain token
a type parameter
an attribute
an operation interface
another defined MLIR mechanism
```

provided that the selected representation preserves all required semantics and
obligations.

It may also define:

- exact operation names;
- exact type syntax;
- exact attribute syntax;
- TableGen class hierarchy;
- C++ class organization;
- builders;
- parser/printer behavior;
- operation interfaces;
- traits;
- canonicalization patterns;
- conversion patterns.

# Forbidden semantic invention in sec_mlir_dialect.md

The detailed dialect specification must not decide or redefine:

```text
what move means
what copy means
what ownership means
what destruction means
when object lifetime begins or ends
when a storage epoch advances
what reclamation authority means
what a safe reference guarantees
what allocation failure means
what layout a type has
what panic means
what Result propagation means
whether an operation may block
what a hardware register access means
```

Those decisions belong to their normative domain rulebooks.

If implementation-oriented work reveals that an existing semantic rule is
insufficient, the domain rulebook must be amended explicitly.

The dialect specification must not silently fill the gap with a new language
rule.

# Relationship to mlir.txt

`mlir.txt` remains the overall MLIR integration and lowering rulebook.

After `sec_mlir.md` and `sec_mlir_dialect.md` are established, `mlir.txt` must be
synchronized so that it does not define a competing Sec dialect architecture.

High-level Sec dialect architecture belongs here.

Exact dialect surface belongs to `sec_mlir_dialect.md`.

General MLIR integration, toolchain boundaries, LLVM translation, and pipeline
integration remain suitable concerns for `mlir.txt`.

Where older `mlir.txt` wording conflicts with this rulebook, it must be updated.

Examples of areas requiring synchronization include:

- the role of the Sec-specific dialect;
- the logical lowering stages;
- unit erasure;
- resolved layout consumption;
- ownership/lifetime/storage ordering;
- runtime-check lowering;
- full-lowering requirements.

# Relationship to mlir-optimize.txt

`mlir-optimize.txt` records optimization concerns and known-correct but
potentially inefficient lowering paths.

It must be synchronized with this architecture.

In particular:

- correctness precedes optimization;
- optimization must preserve all Sec effects;
- resolved layout must be consumed rather than independently recomputed;
- safe-reference, ownership, lifetime, and storage guarantees must constrain
  transformations;
- protected accesses must not be moved outside required protection;
- no optimization may be required for source-level correctness.

Optimization documentation must not become a competing semantic specification.

# Relationship to Semantic IR synchronization

The Semantic IR rulebook may contain older descriptions that predate the final
storage, layout, string-concatenation, and Sec MLIR architecture.

It must be synchronized separately.

That synchronization should include, where applicable:

```text
StorageDomain
invalidation domains
epochs/generations
runtime-generation requirements
ResolvedLayout
object lifetime versus storage lifetime
pin/reclamation protection
StringConcatPlan
unit erasure boundary
updated effect obligations
```

The synchronization must preserve the rule that Semantic IR remains the
canonical semantic representation before MLIR lowering.

# Full-lowering invariant

Normal LLVM emission requires all runtime-relevant Sec-only semantics to have
been discharged or lowered.

Before LLVM translation:

- no high-level Sec operation may remain without a defined lower representation;
- no Sec-specific type may remain if its runtime semantics are unresolved;
- no safety obligation may survive only as undocumented compiler knowledge;
- no ownership responsibility may be ambiguous;
- no required destruction may remain unresolved;
- no runtime check may remain required but unmaterialized;
- no generation/protection requirement may remain unhandled;
- no ABI-relevant high-level representation may remain unresolved.

Debug or provenance metadata may survive when it is not required for runtime
correctness.

Failure of the full-lowering invariant is a compiler error.

# Summary of mandatory architecture

The following rules summarize the Sec MLIR architecture.

```text
Sec Semantic IR is the canonical representation of validated Sec semantics.

Sec MLIR is a lowering and transformation representation. It must not redefine
or infer language semantics that were not established before import.

The Sec compiler uses one `sec` MLIR dialect in version 0.1.

Sec-specific types and operations are introduced only where using standard
MLIR directly would lose, obscure, prematurely erase, or prevent verification
of required Sec semantics.

Sec dialect operations may coexist with standard MLIR dialect operations.

A Sec semantic property may be erased only after its obligations have been
proven statically, represented explicitly at a lower level, or materialized as
required runtime behavior.

Initial Sec MLIR must remain sufficiently close to Semantic IR that remaining
semantic obligations can be identified and verified without reconstructing
source-language intent.

Sec MLIR is target-aware and consumes the active CompilationPlan and resolved
layouts, but remains independent of final callable ABI representation until ABI
lowering.

Resolved layout is consumed by Sec MLIR lowering; MLIR lowering must not define
a competing Sec layout model.

Sec MLIR distinguishes runtime value types, immutable semantic attributes,
semantic operations, and compiler analysis facts.

Stable facts should normally be represented declaratively. Runtime and
control-flow-dependent state must not be represented as immutable compile-time
facts.

Semantic events remain explicit operations until their obligations have been
discharged or materialized.

Compiler analysis results are not automatically persistent IR metadata.

No lowering pass may depend on undocumented semantic information encoded only
by compiler construction convention.

Sec semantic regions and MLIR regions are distinct concepts.

Storage-domain identity, address identity, and generation are distinct
concepts.

Safe references remain semantically qualified until reference-validity
requirements unavailable to ordinary machine pointers have been discharged or
materialized.

Value ownership, backing-storage ownership, reclamation authority, and pointer
representation remain independent concepts.

Units are compile-time semantic constraints and need not survive into Sec MLIR
after all unit relations and conversions have been resolved.

Result, Option, and tagged-union semantics may remain high-level after their
physical layout is known.

String semantics, including concatenation and formatting plans, may remain
high-level until runtime and representation lowering.

Hardware and MMIO semantics remain explicit until target-aware lowering.

Concurrency and reclamation dependencies remain explicit whenever they
constrain legal transformation or access.

Sec effect semantics are defined by effect_analysis.md and must be exposed to
MLIR sufficiently to prevent invalid optimization.

The Sec MLIR dialect is organized into semantic operation families inside one
`sec` dialect.

Standard MLIR operations and types are preferred whenever they preserve the
remaining Sec semantics exactly.

Ownership, object lifetime, storage, allocation, references, and reclamation
are independent semantic concerns.

Checks use a common architectural model based on condition, failure policy,
effects, and provenance.

Result/error flow and panic flow remain distinct until explicitly lowered.

Custom operation interfaces are preferred for cross-cutting semantic queries.

Traits are used for local structural properties and do not substitute for
control-flow-sensitive analysis.

Sec MLIR verification is divided into local, function/region, and
module/whole-program invariant verification.

Every semantically significant lowering stage has a defined verifiable IR
contract.

Canonicalization may simplify semantics but must never invent a new failure
policy, ownership relationship, storage origin, or validity guarantee.

Observable Sec effects constrain dead-code elimination and reordering even when
ordinary memory effects are absent.

Folding, canonicalization, and lowering are distinct transformations.

Sec lowering is progressive and partial rather than one monolithic direct
Sec-to-LLVM conversion.

Each lowering stage may define Sec operations and types as legal, dynamically
legal, or illegal.

Sec MLIR is implemented progressively according to semantic dependencies.

High-level semantic operations must be implemented and verifiable before their
lowering is considered complete.

LLVM lowering is the final stage and must not become a permanent
feature-specific bypass around the Sec dialect.

The Sec MLIR dialect has an internal version independent of the Sec language
version.

Sec 0.1 does not guarantee stable serialized Sec MLIR compatibility between
compiler versions.

Incompatible serialized dialect versions must be rejected deterministically.

Production-ready dialect functionality requires verification, positive tests,
negative verifier tests, lowering tests, and end-to-end behavior tests.

Intermediate lowering stages are legitimate compiler artifacts and must be
individually dumpable and verifiable.

Sec MLIR 0.1 is complete when every code-generating Sec 0.1 semantic feature can
be represented without loss, verified, progressively lowered, and emitted
without permanent feature-specific direct-to-LLVM bypasses.

`sec_mlir_dialect.md` is an exact implementation specification and may choose
local MLIR representation details only within the architecture established by
this rulebook.

The detailed dialect specification must not redefine language, storage,
ownership, lifetime, layout, effect, panic, or reference semantics.
```
