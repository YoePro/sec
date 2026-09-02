# Unsafe

- Status: Normative
- Created: 2026-09-02
- Last updated: 2026-09-02
- Document revision: 2.0
- Sec language version: 0.1
- Canonical path: `rules/memory/unsafe.md`
- Replaces: previous revision of `rules/memory/unsafe.md`
- Repository baseline reviewed: `814a584` (latest publicly verifiable `main`; current `main` contents reviewed 2026-09-02)

---

## § 1 Purpose and authority

§ 1(1) This rulebook defines Sec 0.1 unsafe contexts, unsafe functions, unsafe foreign calls, caller proof obligations, trusted declarations, safe wrappers, and trust provenance.

§ 1(2) Unsafe applies to operations and caller/trust contracts.

§ 1(3) Sec 0.1 does not define:

```text
unsafe struct
unsafe type
```

§ 1(4) A data shape containing raw pointers or foreign handles is not automatically an unsafe type.

§ 1(5) `raw_pointers.md` owns `RawPtr[T]` semantics and canonical raw-pointer operations.

§ 1(6) Ownership, borrowing, lifetime, references, storage, layout, allocation, destruction, transferability, panic, effects, FFI, hardware, ISR, and target rules remain active inside unsafe code.

§ 1(7) Inline assembly has its own canonical rulebook; this book defines its unsafe/trust boundary relationship.

§ 1(8) Unsafe introduces no mandatory runtime mechanism.

---

## § 2 Core principle

§ 2(1) `unsafe` disables no compiler analysis.

§ 2(2) `unsafe` permits a compiler-known operation whose complete safety obligations cannot be proven automatically.

§ 2(3) The programmer accepts only the specific proof obligations attached to that operation or unsafe callable contract.

§ 2(4) Unsafe does not mean:

```text
stop type checking
stop ownership checking
stop borrow checking
stop lifetime analysis
stop effect analysis
stop contract checking
ignore moves or invalidation
ignore cleanup rules
ignore target rules
permit arbitrary backend undefined behavior
```

§ 2(5) The compiler continues every analysis that remains applicable.

§ 2(6) A compiler-proven false safety obligation is rejected even inside unsafe.

---

## § 3 Terminology

### § 3.1 Unsafe operation

§ 3.1(1) An unsafe operation is one whose safety depends on conditions the compiler cannot fully prove at the ordinary safe call site.

Examples include:

```text
raw-pointer dereference
raw-pointer write
raw-pointer arithmetic
raw-to-safe reference construction
slice construction from raw parts
call to unsafe fn
call to unsafe extern
inline assembly
unchecked representation construction
unchecked enum/union construction
raw numeric address interpretation
ownership adoption from raw/foreign storage
```

### § 3.2 Unsafe context

§ 3.2(1) An unsafe context is a lexically explicit source context in which unsafe operations may be written.

§ 3.2(2) It does not make ordinary surrounding operations unchecked.

### § 3.3 Unsafe function

§ 3.3(1) An unsafe function/method is a Sec callable whose caller must satisfy additional proof obligations that ordinary type/semantic checking does not fully establish.

### § 3.4 Trusted declaration

§ 3.4(1) A trusted declaration is one whose implementation or target fact lies outside ordinary Sec source verification.

Examples include:

```text
foreign functions
inline assembly contracts
raw numeric target addresses
compiler intrinsics
target knowledge-pack facts
platform-provided ABI/layout facts
```

---

## § 4 Unsafe context syntax

§ 4(1) Sec supports both single-operation and block forms.

§ 4(2) Canonical single-operation expression form:

```sec
let value := unsafe pointer.Read()
```

§ 4(3) Canonical single-operation statement form:

```sec
unsafe pointer.Write(value)
```

§ 4(4) Canonical block form:

```sec
let value := unsafe {
    Validate(pointer)
    pointer.Read()
}
```

§ 4(5) An unsafe block follows ordinary block-expression result semantics.

§ 4(6) The `unsafe` marker applies only to the syntactic operation/block it introduces.

§ 4(7) Parent scopes do not become implicitly unsafe.

§ 4(8) Nested unsafe contexts are permitted but may be diagnosed as redundant where no additional boundary clarity is gained.

---

## § 5 Empty and redundant unsafe contexts

§ 5(1) An empty unsafe block is valid syntax.

§ 5(2) An unsafe context containing no operation requiring unsafe is valid.

§ 5(3) By default such contexts produce informational diagnostics rather than hard errors.

§ 5(4) Tooling/project policy may promote redundant/empty unsafe diagnostics to warnings.

§ 5(5) Removing a redundant unsafe marker must not change semantics.

---

## § 6 `unsafe fn`

§ 6(1) Sec supports unsafe functions and methods.

Canonical form:

```sec
unsafe fn FromRawParts(
    pointer: RawPtr[byte],
    length: int,
) Buffer {
    // ...
}
```

§ 6(2) Calling an unsafe function requires an unsafe context.

```sec
let buffer := unsafe Buffer.FromRawParts(pointer, length)
```

§ 6(3) Unsafe function status is part of the callable safety contract.

§ 6(4) Unsafe status is not a claim about return type or error behavior.

§ 6(5) Unsafe status is not equivalent to `MayPanic`, `MayAllocate`, `MayBlock`, or any other effect.

---

## § 7 Unsafe function body remains explicit

§ 7(1) The body of an `unsafe fn` is not implicitly an unsafe context.

§ 7(2) Each actual unsafe operation inside it remains explicitly marked.

```sec
unsafe fn ReadRaw(pointer: RawPtr[byte]) byte {
    return unsafe pointer.Read()
}
```

§ 7(3) This rule localizes trust and makes the exact implementation operation reviewable.

§ 7(4) An unsafe function may contain no unsafe operation when its caller obligations are required for a safe internal operation or future implementation strategy.

§ 7(5) Such a function remains unsafe to call because the public contract carries proof obligations.

---

## § 8 Caller proof obligations

§ 8(1) Every unsafe callable must have defined caller obligations.

§ 8(2) Obligations may include:

```text
pointer non-null
pointer aligned
valid address range
storage live for a stated lifetime
initialized representation
exclusive mutable authority
no conflicting aliases
correct ownership/deallocation contract
valid target address space
correct hardware access context
correct foreign ABI
callback/thread affinity
no concurrent invalidation
length/capacity relationship
```

§ 8(3) The compiler may verify some obligations at a particular call site.

§ 8(4) Verified obligations need not be treated as unknown merely because the callable is unsafe.

§ 8(5) Remaining obligations are accepted by the explicit unsafe context.

§ 8(6) A compiler-proven violation remains a compile-time error.

---

## § 9 Safety documentation

§ 9(1) Public unsafe APIs should document caller obligations in source documentation.

§ 9(2) Safety documentation is strongly recommended rather than a hard language requirement in Sec 0.1.

§ 9(3) Tooling may diagnose missing safety documentation for exported/public unsafe APIs where a project policy requests it.

§ 9(4) Compiler-known unsafe operations must have machine-readable obligation metadata independent of comments.

---

## § 10 `unsafe extern`

§ 10(1) Canonical foreign form:

```sec
unsafe extern "system" fn rawSysCall(
    number: int,
    argument1: uint64,
) int64
```

§ 10(2) `unsafe`, `extern "system"`, and `fn` have separate meanings.

§ 10(3) `unsafe` specifies caller proof obligations.

§ 10(4) `extern "system"` specifies foreign linkage/ABI selection.

§ 10(5) `fn` declares a callable function.

§ 10(6) `unsafe extern` is not one indivisible keyword.

§ 10(7) Other linkage forms are defined by the FFI rulebook.

§ 10(8) C `...` varargs remain foreign ABI syntax only where FFI rules explicitly support them.

---

## § 11 Calling unsafe extern functions

§ 11(1) Calling an unsafe extern function requires unsafe context.

```sec
let result := unsafe rawSysCall(number, argument)
```

§ 11(2) The marker does not establish correct arguments, pointer validity, ownership, foreign lifetime, effect claims, or ABI declaration.

§ 11(3) Those obligations belong to the declaration/call contract.

§ 11(4) Foreign undefined behavior caused by a false contract is not repaired by Sec's unsafe marker.

---

## § 12 Safe wrappers

§ 12(1) A safe function may use unsafe operations internally and expose a safe API when it validates or establishes every caller obligation.

```sec
fn CreateBuffer(
    pointer: RawPtr[byte],
    length: int,
) Result[Buffer, BufferError] {
    if pointer.IsNull() {
        return Err(BufferError.NullPointer)
    }

    if length < 0 {
        return Err(BufferError.InvalidLength)
    }

    let buffer := unsafe Buffer.FromRawParts(pointer, length)
    return Ok(buffer)
}
```

§ 12(2) A safe wrapper must not leave hidden caller obligations that its safe type/API contract cannot express or enforce.

§ 12(3) Recoverable validation failures should use ordinary `Result`/Option/control flow rather than hidden panic where appropriate.

§ 12(4) A safe wrapper may rely on compiler-proven target/platform facts.

---

## § 13 Unsafe does not propagate through safe wrappers

§ 13(1) Unsafe is not a transitive caller marker.

§ 13(2) A correct safe wrapper remains safe to call.

§ 13(3) Actual effects remain transitive.

§ 13(4) A safe wrapper around a blocking foreign call remains blocking.

§ 13(5) A safe wrapper around an allocating intrinsic remains allocating.

§ 13(6) A safe wrapper around a panic-capable operation remains panic-capable unless panic is proven absent/handled by canonical semantics.

---

## § 14 Trust provenance

§ 14(1) A safe wrapper does not erase internal trust provenance.

§ 14(2) The compiler may record:

```text
caller-facing safety: safe
implementation provenance: depends on unsafe foreign call
```

§ 14(3) Trust provenance may support auditing, security review, ISR reports, LSP navigation, whole-program trust reports, diagnostics, and certification profiles.

§ 14(4) Trust provenance is not automatically part of the source-level callable type.

§ 14(5) Trusted facts must identify their provenance sufficiently to invalidate/review them when the target/FFI/platform contract changes.

---

## § 15 Effects remain active

§ 15(1) Unsafe does not suppress effects.

§ 15(2) Effects may include:

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

§ 15(3) An unsafe operation may have multiple effects.

§ 15(4) Unsafe status and effect set are separate semantic dimensions.

§ 15(5) `@noPanic`, `@noAlloc`, `@noBlock`, ISR, and other verified effect contracts continue to apply.

---

## § 16 Unsafe and panic

§ 16(1) Unsafe does not make panic-capable code compatible with `@noPanic`.

§ 16(2) A trusted foreign no-panic claim may provide proof only when the FFI contract explicitly supports such trust.

§ 16(3) Trust provenance must remain attached to accepted external claims.

§ 16(4) Unsafe cleanup that must run on panic remains subject to panic/destruction no-panic requirements.

---

## § 17 Unsafe and allocation

§ 17(1) Unsafe does not authorize hidden allocation.

§ 17(2) Raw-pointer/reference conversion, ownership adoption, or closure escape must not allocate merely to make the operation legal.

§ 17(3) Allocation-capable unsafe intrinsics retain `MayAllocate`.

§ 17(4) `@noAlloc`/ISR allocation restrictions remain enforced.

---

## § 18 Unsafe and blocking/suspension

§ 18(1) Unsafe does not authorize blocking or suspension in a context that forbids them.

§ 18(2) Foreign/assembly/target operations must publish blocking/suspension effects or be treated conservatively.

§ 18(3) An ISR cannot call an unsafe blocking operation merely because it is marked unsafe.

---

## § 19 Raw-pointer storage and value operations

§ 19(1) Merely storing, copying, moving, returning, passing, equality-testing, or null-testing `RawPtr[T]` is not inherently unsafe when `raw_pointers.md` defines the operation as safe.

§ 19(2) These operations do not assert pointee validity.

§ 19(3) A type containing `RawPtr[T]` is not automatically unsafe.

---

## § 20 Raw-pointer interpretation operations

§ 20(1) Canonical raw-pointer interpretation operations requiring unsafe include:

```sec
pointer.Read()
pointer.Write(value)
pointer.VolatileRead()
pointer.VolatileWrite(value)
pointer.Offset(elements)
pointer.AddBytes(bytes)
pointer.Difference(other)
```

where the dedicated raw-pointer rule classifies the operation as unsafe.

§ 20(2) Integer-to-pointer construction, raw-to-safe reference construction, slice-from-raw-parts, and ownership adoption also require unsafe.

§ 20(3) Exact safe-reference/RawPtr conversion spelling remains owned by the canonical conversion/raw-pointer rule and must not be invented by this book.

---

## § 21 Raw-pointer obligations

§ 21(1) Unsafe raw reads/writes require the complete relevant non-null, alignment, lifetime, representation, range, address-space, alias, ownership, and target obligations from `raw_pointers.md`.

§ 21(2) Unsafe context does not make null dereference valid.

§ 21(3) Unsafe context does not make a stale/dangling known address valid.

§ 21(4) Unsafe context does not make misaligned safe-reference construction valid.

§ 21(5) Unsafe context does not permit manufacturing conflicting `ref mut` values.

---

## § 22 Raw-to-safe reference construction

§ 22(1) Constructing a safe reference from raw storage requires unsafe unless a specialized trusted adapter proves all guarantees.

§ 22(2) Required shared-reference obligations include at least:

```text
non-null
alignment
valid initialized T
readable storage
lifetime covers reference
canonical provenance established
no conflicting mutable access
foreign/hardware mutation compatible with shared semantics
address-space/platform compatibility
```

§ 22(3) `ref mut T` additionally requires writable storage and exclusive mutable authority for the complete borrow live range.

§ 22(4) Compiler-proven contradictions are rejected even in unsafe.

---

## § 23 Slice from raw parts

§ 23(1) Constructing a safe slice/view from raw pointer plus extent is unsafe unless a trusted wrapper establishes the complete contract.

§ 23(2) Obligations include range, nonnegative/representable extent, multiplication overflow, alignment, element initialization, lifetime, alias/mutability, one compatible storage region where required, and target address-space validity.

§ 23(3) Slice construction must not introduce hidden allocation.

§ 23(4) A zero-length slice does not permit fabricating an arbitrary valid scalar reference.

---

## § 24 Unchecked representation construction

§ 24(1) Creating typed values from raw bits/storage without ordinary constructors is unsafe unless a compiler-verified representation operation proves validity.

§ 24(2) Unsafe does not make every bit pattern a valid value.

§ 24(3) Obligations may include enum discriminants, union active variant, bool/rune validity, named-type contracts, reference invariants, ownership state, alignment, padding, and FFI layout.

§ 24(4) Compiler-proven invalid representation is rejected.

---

## § 25 Unchecked enum/union operations

§ 25(1) Constructing an enum from an unverified underlying value may require unsafe when the value may not name a valid enum member/state.

§ 25(2) Constructing/selecting a union payload/tag without ordinary validated constructors/match state may require unsafe.

§ 25(3) Unsafe operations must preserve active-variant/destruction semantics or explicitly transfer the corresponding proof obligations.

§ 25(4) Inactive union payload reads remain invalid.

---

## § 26 Ownership adoption

§ 26(1) Adopting ownership from raw/foreign storage is unsafe unless a specialized safe wrapper establishes the ownership contract.

§ 26(2) Obligations include correct allocator/provider, matching reclamation operation, extent/alignment, uniqueness, initialized objects, destruction responsibilities, and foreign lifetime/retention rules.

§ 26(3) Numeric address alone never proves ownership.

§ 26(4) Double adoption of the same exclusive resource is invalid.

§ 26(5) Unsafe does not permit double-free/double-destruction.

---

## § 27 Ownership release to foreign code

§ 27(1) Transferring Sec ownership to foreign code requires an explicit FFI ownership contract.

§ 27(2) Raw pointer passing alone does not imply ownership transfer.

§ 27(3) After a committed consuming foreign transfer, the Sec source Place becomes unavailable according to ownership rules.

§ 27(4) Failure/non-commit behavior must be defined by the foreign wrapper contract.

---

## § 28 Unsafe and borrowing

§ 28(1) Unsafe does not disable borrow checking for safe references.

§ 28(2) Raw aliases may exist outside the safe borrow model, but using them in ways that violate live safe-reference guarantees is invalid.

§ 28(3) Raw-to-safe reconstruction must re-establish borrow compatibility.

§ 28(4) Known overlap/conflict is rejected.

§ 28(5) Unknown external mutation may make a safe reference wrapper impossible unless the contract provides synchronization/exclusivity guarantees.

---

## § 29 Unsafe and lifetime

§ 29(1) Unsafe does not extend storage/object/reference lifetime.

§ 29(2) A raw pointer may outlive storage, but dereference after lifetime end is invalid.

§ 29(3) Safe wrappers must not return references whose lifetime depends on ended local storage.

§ 29(4) Unsafe must not trigger hidden heap/Arena promotion to repair lifetime.

§ 29(5) Known use-after-domain-end is rejected even inside unsafe.

---

## § 30 Unsafe and destruction

§ 30(1) Unsafe does not disable deterministic destruction.

§ 30(2) Raw writes must not overwrite a live non-trivial object while bypassing required destruction unless an explicit low-level replacement/adoption contract proves the lifecycle transition.

§ 30(3) Unsafe operations inside `defer`, `free`, or other cleanup remain explicitly marked where permitted.

§ 30(4) `free` restrictions from `destruction.md` remain active.

§ 30(5) Unsafe does not make panic-capable required cleanup legal.

---

## § 31 Unsafe and transferability/concurrency

§ 31(1) Unsafe does not make a non-transferable value transferable.

§ 31(2) Unsafe does not legalize a data race.

§ 31(3) Unsafe does not waive thread affinity, task migration, process boundary, ISR, or destruction-context restrictions.

§ 31(4) Volatile access is not synchronization.

§ 31(5) Foreign/raw concurrency requires explicit synchronization/ownership contracts.

---

## § 32 FFI trust boundary

§ 32(1) Foreign declarations may be trusted/unsafe because Sec cannot verify their implementation.

§ 32(2) FFI contracts should publish:

```text
ABI/linkage
nullability
ownership transfer
borrowing/retention
lifetime
thread/callback affinity
allocation/deallocation
panic/abort/unwind behavior
blocking/suspension
I/O/external mutation
layout/alignment/extent
```

§ 32(3) Unknown foreign behavior is conservative where proof is required.

§ 32(4) Foreign exceptions/unwinding must not cross Sec frames unless a dedicated FFI rule explicitly permits/proves it.

---

## § 33 Safe foreign wrappers

§ 33(1) Safe wrappers may translate foreign null/error/status results into typed Sec `Option`/`Result`.

§ 33(2) Safe wrappers may construct safe references/owners only after establishing every required FFI/raw/storage/lifetime contract.

§ 33(3) Safe wrappers must preserve actual effects and trust provenance.

§ 33(4) A safe wrapper must not claim cross-thread/ISR safety that the foreign contract does not provide.

---

## § 34 Inline assembly

§ 34(1) Inline assembly is an unsafe operation and trust boundary.

§ 34(2) It requires explicit unsafe context unless its dedicated grammar includes an equivalent explicit unsafe marker.

§ 34(3) Assembly contracts must describe enough behavior for compiler correctness.

§ 34(4) At minimum the model must support register inputs/outputs/clobbers, memory reads/writes, volatile behavior, control flow, stack behavior, possible trap/abort, blocking, I/O, and external mutation.

§ 34(5) Exact assembly syntax belongs to the inline-assembly rulebook.

§ 34(6) Assembly effects remain active inside unsafe.

---

## § 35 Target knowledge and fixed addresses

§ 35(1) Compiler-verified target knowledge is not inherently unsafe merely because it describes hardware.

§ 35(2) `@address` is not automatically prefixed with `unsafe`.

§ 35(3) A canonical target-known peripheral/address may be validated by the compiler/platform contract.

§ 35(4) Raw numeric address interpretation requires unsafe when the compiler cannot prove the complete storage/access contract.

§ 35(5) Unknown/unverified target facts retain trust provenance or are rejected by target policy.

---

## § 36 Hardware register access

§ 36(1) Safe typed hardware register access may be provided by compiler/platform-verified declarations.

§ 36(2) Raw-pointer MMIO access remains unsafe according to raw-pointer/volatile rules.

§ 36(3) Unsafe does not bypass exact access width, volatile ordering, mapping, device, privilege, or ISR restrictions.

§ 36(4) Hardware signal polarity remains application/driver semantics.

---

## § 37 Runtime mappings

§ 37(1) Mapping creation/destruction is governed by platform mapping/resource rules.

§ 37(2) Unsafe raw access inside a mapping requires mapping lifetime/extent/address-space validity.

§ 37(3) Raw pointers do not keep mappings alive.

§ 37(4) Remap/unmap can invalidate prior raw/safe views.

§ 37(5) Safe mapping wrappers may encapsulate raw OS/platform operations.

---

## § 38 Interrupt vectors and ISR

§ 38(1) Named target-known interrupt vectors may be compiler-verified without unsafe.

§ 38(2) Raw numeric vector/address use is validated by target rules and may require trust/unsafe boundaries.

§ 38(3) Unsafe does not waive `@isr` or `@interruptSafe` `noPanic`, `noAlloc`, `noBlock`, bounded-work, synchronization, or hardware-access requirements.

§ 38(4) Unsafe helper calls reachable from ISR must publish complete effects/trust facts.

---

## § 39 Function values

§ 39(1) Unsafe status is part of callable compatibility.

§ 39(2) An unsafe function must not implicitly convert to a safe function value.

§ 39(3) A safe wrapper may produce a safe callable after discharging caller obligations.

§ 39(4) Calling through an unsafe function value requires unsafe context.

§ 39(5) Effect and trust summaries remain attached to the callable value/target set.

---

## § 40 Interfaces

§ 40(1) Interface callable safety contracts must be preserved by implementations.

§ 40(2) A safe interface method must not be implemented by a callable requiring additional unsafe caller obligations.

§ 40(3) An unsafe interface method may be implemented internally with safe code while remaining unsafe to call through the interface contract.

§ 40(4) Dynamic dispatch must not erase unsafe status.

§ 40(5) Unknown dynamic target safety is conservative.

---

## § 41 Generics

§ 41(1) Generic code must preserve unsafe requirements of operations on instantiated types/callables.

§ 41(2) A generic constraint must not silently assume a callback/operation is safe when safety status is unknown.

§ 41(3) Specialization may prove an operation safe and remove an unnecessary unsafe boundary only when canonical semantics permit it.

§ 41(4) Trust provenance from unsafe generic dependencies remains available for analysis/tooling.

---

## § 42 Closures and captures

§ 42(1) Unsafe status of a called operation is independent of closure capture ownership.

§ 42(2) A closure may capture `RawPtr` safely as a value while later dereference remains unsafe.

§ 42(3) Closure transferability/lifetime remains checked normally.

§ 42(4) A closure invoking an unsafe callable must do so in explicit unsafe context.

§ 42(5) Safe closure wrappers may encapsulate unsafe operations only when all caller/environment obligations are established.

---

## § 43 Properties, init, and lifecycle bodies

§ 43(1) Unsafe operations in property accessors, `init`, methods, functions, and other ordinary executable bodies require explicit unsafe context.

§ 43(2) Unsafe does not create new locations where a syntactic construct is otherwise forbidden.

§ 43(3) `free` restrictions remain governed by destruction rules; unsafe does not make prohibited defer/escape behavior legal.

---

## § 44 Compile-time evaluation

§ 44(1) Ordinary compile-time evaluation rejects unsafe operations.

§ 44(2) The compiler does not ordinarily execute raw-pointer dereference, FFI call, inline assembly, addressed-storage access, raw target memory access, or unsafe ownership adoption during compile-time evaluation.

§ 44(3) A compiler-known intrinsic may be compile-time evaluable only when explicitly specified.

§ 44(4) Unsafe context does not force compile-time execution.

§ 44(5) Compile-time rejection is independent from runtime safety.

---

## § 45 Backend undefined behavior

§ 45(1) Sec should minimize backend undefined behavior.

§ 45(2) Unsafe is not permission to lower arbitrary source into unrestricted backend UB.

§ 45(3) Where practical, unsafe operations should lower to defined target instructions, explicit traps, validated intrinsics, well-specified foreign ABI operations, or carefully bounded backend assumptions.

§ 45(4) When a false unsafe obligation necessarily violates backend assumptions, documentation/tooling must describe that risk honestly.

§ 45(5) Backend assumptions must be no stronger than the obligations/trusted facts accepted at the source/Semantic IR boundary.

---

## § 46 No mandatory runtime

§ 46(1) Unsafe is a compile-time language/analysis feature.

§ 46(2) It requires no runtime unsafe flags, dynamic trust checks, mandatory pointer metadata, mandatory provenance objects, mandatory generational references, exception machinery, or general Sec runtime.

§ 46(3) Profiles may independently use runtime checks for other safety models.

§ 46(4) Trust provenance may be erased from release binaries where not required by the selected diagnostics/certification profile.

---

## § 47 Canonical unsafe-operation registry

§ 47(1) The compiler should maintain one canonical registry or equivalent shared source of unsafe operation definitions.

§ 47(2) Each operation entry should define:

```text
stable identity
source operation/member
required unsafe context
caller obligations
effects
trust provenance
allowed targets
diagnostics
safe alternatives where available
```

§ 47(3) Unsafe classification must not be distributed as unrelated ad-hoc checks.

§ 47(4) Compiler and LSP must consume the same registry.

---

## § 48 Semantic representation

§ 48(1) Sema/Semantic IR preserve where required:

```text
unsafe operation kind
lexical unsafe context
unsafe callable contract
caller obligations
trust provenance
effect summary
source location
resolved target/foreign facts
safe-wrapper boundary
```

§ 48(2) Unsafe and effect properties must not be stored as one boolean.

§ 48(3) A safe wrapper may remain caller-safe while retaining internal trust provenance/effects.

§ 48(4) Contradictory trusted facts must be diagnosed/invalidated before lowering where possible.

---

## § 49 Lowering

§ 49(1) Lowering preserves unsafe operation semantics and target constraints.

§ 49(2) Raw operations lower according to `raw_pointers.md` and target layout/address-space rules.

§ 49(3) Volatile/hardware operations preserve exact physical access semantics.

§ 49(4) FFI calls preserve declared ABI.

§ 49(5) Inline assembly preserves declared clobber/memory/control-flow/effect contracts.

§ 49(6) Unsafe marker itself need not produce runtime code.

§ 49(7) Unused unsafe/FFI helpers need not be linked.

---

## § 50 Diagnostics

§ 50(1) Unsafe diagnostics must follow the mentor-compiler principle.

§ 50(2) Stable diagnostic categories should include at least:

```text
unsafe.required
unsafe.unnecessary
unsafe.redundant
unsafe.unclosed
unsafe.invalid-context
unsafe.call-requires-context
unsafe.operation-requires-context
unsafe.function-body-still-explicit
unsafe.interface-mismatch
unsafe.function-value-mismatch
unsafe.generic-constraint-unknown
unsafe.foreign-effect-unknown
unsafe.foreign-ownership-unknown
unsafe.raw-pointer-null
unsafe.raw-pointer-alignment
unsafe.raw-pointer-lifetime
unsafe.raw-pointer-alias
unsafe.raw-pointer-range
unsafe.uninitialized-read
unsafe.invalid-representation
unsafe.unchecked-construction
unsafe.target-trust
unsafe.address-unverified
unsafe.cleanup-effect
unsafe.compile-time-forbidden
```

§ 50(3) A diagnostic must distinguish missing unsafe context from a proven-invalid operation.

§ 50(4) Diagnostics should show caller obligations and known violated facts.

§ 50(5) Redundant/empty unsafe diagnostics are informational by default.

---

## § 51 Formatter

§ 51(1) Formatter preserves canonical forms:

```sec
unsafe pointer.Write(value)
```

```sec
let value := unsafe {
    pointer.Read()
}
```

```sec
unsafe fn FromRawParts(...) Buffer {
}
```

```sec
unsafe extern "system" fn rawSysCall(number: int, argument1: uint64) int64
```

§ 51(2) Formatter must not add/remove unsafe semantics.

§ 51(3) Modifier order preserves `unsafe extern "<abi>" fn` and `unsafe fn`.

§ 51(4) Formatter must not invent `pub`, `unsafe struct`, or `unsafe type`.

---

## § 52 LSP and tooling

§ 52(1) LSP should display unsafe-operation explanation, caller obligations, callable safety, foreign trust, effects, trust provenance, nearest unsafe context, safe-wrapper boundary, raw-address verification, target-knowledge source, and callable compatibility where relevant.

§ 52(2) Completion inside unsafe remains ordinary completion with additional marking for operations permitted only by the context.

§ 52(3) Code actions may wrap an exact operation/block in unsafe or remove redundant unsafe when semantics are preserved.

§ 52(4) Tooling must not imply that wrapping a compiler-proven invalid operation in unsafe will fix it.

§ 52(5) Compiler/LSP/sec analyse must agree.

---

## § 53 Required test families

§ 53(1) Syntax tests include single-operation statement/expression, block expression, block result, unclosed block, empty/redundant unsafe, unsafe fn/method, and unsafe extern linkage forms.

§ 53(2) Unsafe-function tests include call outside context rejected, calls inside both context forms accepted, body remains explicit, safe wrapper accepted, and effects/trust preserved.

§ 53(3) Callable compatibility tests include interfaces, function values, callbacks, dynamic dispatch, and generic unknown safety.

§ 53(4) Raw-pointer tests include read/write/volatile/arithmetic gating, null/alignment/lifetime/alias/range obligations, RawPtr storage operations, and raw-to-safe conversion.

§ 53(5) Construction tests include invalid representation, unchecked enum/union, uninitialized storage, ownership adoption, and double ownership rejection.

§ 53(6) FFI tests include ABI, ownership/retention/effects, unknown contracts, safe wrappers, and foreign unwind restrictions.

§ 53(7) Hardware tests include knowledge-pack address, raw numeric address, misalignment, device mapping lifetime, MMIO, ISR effects, and volatile-not-synchronization.

§ 53(8) Compile-time tests reject ordinary unsafe operations except explicitly supported intrinsics.

§ 53(9) Binary tests verify no runtime unsafe flag/general runtime and direct target lowering of used operations.

§ 53(10) Tooling tests verify diagnostics/formatter/LSP parity.

---

## § 54 Completion criteria

§ 54(1) Frontend unsafe support is complete when both unsafe-context forms, unsafe fn/method, unsafe extern, unsafe-call checking, and all canonical operation classifications are implemented.

§ 54(2) Contract support is complete when caller obligations and trust provenance are compiler-owned facts across direct/indirect/interface/generic/FFI calls.

§ 54(3) Raw/representation support is complete when every unsafe low-level operation consumes the canonical raw/layout/storage/ownership/reference facts.

§ 54(4) Effect integration is complete when unsafe never suppresses panic/allocation/blocking/volatile/I/O/external-mutation or other effects.

§ 54(5) Platform/FFI support is complete when target knowledge, raw addresses, mappings, hardware, assembly, callbacks, and foreign contracts preserve trust/effect provenance.

§ 54(6) Semantic IR/lowering support is complete when unsafe operations lower with no invented runtime and no stronger backend assumptions than accepted proof/trust.

§ 54(7) Tooling support is complete when compiler, LSP, formatter, diagnostics, and audit/report tooling consume one canonical unsafe registry.

---

## § 55 Core summary

§ 55(1) Unsafe permits an operation whose complete safety obligations are not automatically proven; it disables no compiler analysis.

§ 55(2) Sec supports `unsafe operation` and `unsafe { ... }`.

§ 55(3) Sec supports `unsafe fn` and `unsafe extern "<abi>" fn`.

§ 55(4) Calling an unsafe callable requires unsafe context.

§ 55(5) The body of `unsafe fn` is not implicitly unsafe; each unsafe operation remains explicit.

§ 55(6) Safe wrappers may encapsulate unsafe implementation details after discharging all caller obligations.

§ 55(7) Unsafe does not transitively mark callers, but actual effects/trust provenance remain visible.

§ 55(8) Raw-pointer interpretation, raw-to-safe conversion, slice-from-raw-parts, unchecked representation, ownership adoption, FFI, assembly, and unverified target addressing are principal unsafe boundaries.

§ 55(9) Unsafe does not legalize null/dangling/alias/lifetime/data-race/target violations that the compiler can prove.

§ 55(10) Unsafe is not a general backend-UB switch and requires no mandatory Sec runtime.
