# Raw Pointers

- Status: Normative
- Created: 2026-09-01
- Last updated: 2026-09-01
- Document revision: 2.0
- Sec language version: 0.1
- Canonical path: `rules/memory/raw_pointers.md`
- Replaces: `rules/memory/raw_pointers.txt`
- Repository baseline reviewed: `814a584`

---

## § 1 Purpose and authority

§ 1(1) This rulebook defines `RawPtr[T]`, the Sec type for unchecked typed memory addresses.

§ 1(2) `RawPtr[T]` is intended for:

```text
FFI
operating-system interfaces
hardware and runtime-discovered addresses
allocator implementation
low-level runtime-free platform code
compiler/runtime internals
explicit unsafe memory operations
```

§ 1(3) `RawPtr[T]` is not a safe Sec reference.

§ 1(4) `RawPtr[T]` does not participate in ownership merely because it contains an address.

§ 1(5) `RawPtr[T]` does not extend the lifetime of a value or storage region.

§ 1(6) `RawPtr[T]` validity is not guaranteed by the compiler solely from the raw-pointer value.

§ 1(7) `rules/memory/unsafe.md` owns the general unsafe-context model and caller proof obligations.

§ 1(8) `rules/memory/references.md` and `rules/memory/reference_model.md` own safe-reference semantics, provenance, validity epochs, and safe-reference representation.

§ 1(9) `rules/memory/borrowing.md` owns borrow authority and alias compatibility.

§ 1(10) `rules/memory/lifetime_analysis.md` owns lifetime proof and escaping-lifetime relationships.

§ 1(11) `rules/memory/ownership.md`, `copy_move.md`, `destruction.md`, `allocation.md`, and `storage.md` own their respective memory-model semantics.

§ 1(12) FFI rulebooks own foreign ownership/retention contracts and ABI requirements.

§ 1(13) `rules/platform/fixed-address-bindings.md`, `hardware-register-access.md`, and `volatile.md` own platform-address, mapping, hardware-access, and volatile semantics.

§ 1(14) This rulebook does not invent a separate pointer ownership or lifetime model.

---

## § 2 Core model

§ 2(1) A `RawPtr[T]` value is an address value interpreted as potentially addressing storage suitable for `T`.

§ 2(2) The address may be null.

§ 2(3) The address may be dangling.

§ 2(4) The address may be unaligned.

§ 2(5) The address may refer to inaccessible storage.

§ 2(6) The address may refer to storage whose current representation is not a valid `T`.

§ 2(7) Merely storing, copying, moving, comparing, returning, or passing a `RawPtr[T]` does not assert that any of the conditions in § 2(2)-§ 2(6) are false.

§ 2(8) Unsafe interpretation of a raw address transfers proof obligations to the programmer or trusted wrapper; it does not disable compiler analysis.

§ 2(9) A raw pointer carries no implicit ownership, destruction responsibility, allocation-domain identity, generation validity, bounds, exclusivity, or hardware privilege.

§ 2(10) Additional semantic facts may be attached by compiler/trusted/platform analysis, but they are separate from the bare `RawPtr[T]` type.

---

## § 3 Type syntax

§ 3(1) Raw pointers use the generic type syntax:

```sec
RawPtr[T]
```

§ 3(2) Examples:

```sec
let address: RawPtr[byte]
let handle: RawPtr[void]
```

§ 3(3) `RawPtr[void]` represents an untyped raw address.

§ 3(4) `RawPtr[void]` does not support typed `Read()`, `Write()`, `VolatileRead()`, or `VolatileWrite()` operations without first being converted to a concrete compatible raw-pointer type under the applicable rules.

§ 3(5) Sec does not use C-like raw-pointer type syntax such as:

```text
*T
const T*
void*
```

§ 3(6) `RawPtr[T]` is a compiler-known generic type and must not be shadowed or redefined by user code.

---

## § 4 Raw pointers are values

§ 4(1) A `RawPtr[T]` is an ordinary Sec value with raw-address semantics.

§ 4(2) `RawPtr[T]` is copyable.

§ 4(3) Copying `RawPtr[T]` copies only the address value and any canonical representation metadata required by the selected target.

§ 4(4) Copying a raw pointer does not copy, clone, retain, borrow, or transfer the pointee.

§ 4(5) Moving a raw pointer changes only ownership of the raw-pointer value when an explicit move operation is used; it does not move the pointee.

§ 4(6) Destruction of a `RawPtr[T]` value performs no pointee destruction or deallocation.

§ 4(7) `RawPtr[T]` may be stored in structs, arrays, unions, generic types, and other ordinary value containers.

§ 4(8) A type containing `RawPtr[T]` is not automatically an unsafe type.

---

## § 5 Safe versus unsafe operations

### § 5.1 Operations that may be safe

§ 5.1(1) The following operations do not require unsafe merely because they manipulate `RawPtr[T]`:

```text
store the pointer value
copy the pointer value
move the pointer value
return the pointer value
pass the pointer value through a compatible FFI boundary
compare compatible raw pointers for equality
test a raw pointer for null
format or inspect the address for diagnostics where permitted
perform an explicitly representation-compatible raw-pointer conversion
```

§ 5.1(2) These operations do not dereference or otherwise prove pointee validity.

### § 5.2 Operations that require unsafe

§ 5.2(1) The following require an unsafe context unless a more specialized canonical rule explicitly provides a compiler-verified safe operation:

```text
Read()
Write(value)
VolatileRead()
VolatileWrite(value)
pointer arithmetic
integer-to-RawPtr conversion
constructing ref T from RawPtr[T]
constructing ref mut T from RawPtr[T]
constructing a slice/view from RawPtr plus extent
assuming alignment
assuming initialized representation
adopting ownership from a raw address
constructing a callable function pointer from a raw address
reinterpreting unrelated pointer representations without compiler proof
```

§ 5.2(2) `unsafe` does not make an invalid raw-pointer operation valid.

§ 5.2(3) `unsafe` does not disable type checking, ownership checking of ordinary values, borrow checking of safe references, lifetime analysis, Result handling, effect analysis, target validation, or hardware-access validation.

---

## § 6 Unsafe context syntax

§ 6(1) Raw-pointer operations use the canonical unsafe forms defined by `unsafe.md`.

Single operation:

```sec
let value := unsafe pointer.Read()
```

```sec
unsafe pointer.Write(value)
```

Block:

```sec
let value := unsafe {
    Validate(pointer)
    pointer.Read()
}
```

§ 6(2) The body of an `unsafe fn` is not implicitly an unsafe context.

```sec
unsafe fn ReadRaw(pointer: RawPtr[byte]) byte {
    return unsafe pointer.Read()
}
```

§ 6(3) Unsafe context accepts proof obligations for the operation in its lexical extent only.

---

## § 7 Null

§ 7(1) Null is part of the value domain of every `RawPtr[T]`.

§ 7(2) A raw pointer may be null after:

```text
foreign return
unsafe conversion
platform operation
allocator or operating-system operation
another raw-pointer-producing operation
```

§ 7(3) Storing, moving, returning, passing, comparing, or testing a null raw pointer is permitted where the surrounding operation is otherwise valid.

§ 7(4) Safe references remain non-null.

§ 7(5) Conversion from `RawPtr[T]` to a safe reference requires proof of non-nullness in addition to all other reference obligations.

§ 7(6) Dereferencing a null raw pointer is invalid.

§ 7(7) `unsafe` does not make null dereference valid.

§ 7(8) A compiler-proven null dereference is a compile-time error even inside unsafe code.

§ 7(9) The source sentinel `null` is restricted by the canonical FFI/raw-pointer grammar rules; that syntax restriction does not narrow the runtime value domain of `RawPtr[T]`.

§ 7(10) Safe wrappers should normally convert nullable foreign outcomes into explicit abstractions such as:

```sec
Option[RawPtr[T]]
Result[RawPtr[T], E]
```

or a validated wrapper type.

---

## § 8 Construction

§ 8(1) `RawPtr[T]` may originate from:

```text
foreign function result
explicit unsafe safe-reference conversion
explicit unsafe integer/address conversion
platform or hardware address primitive
allocator/runtime primitive
another compatible RawPtr conversion
```

§ 8(2) A raw pointer must never be created implicitly from a safe reference, integer, owning container, string, slice, or unrelated pointer representation.

§ 8(3) Exact source spelling for safe-reference-to-raw and raw-to-safe reference conversion is not newly defined by this v2 rewrite.

§ 8(4) Until a dedicated conversion spelling is canonically locked, implementations must not treat illustrative conversion syntax as normative language grammar.

§ 8(5) Existing compiler-known raw operations and already canonical unsafe syntax remain normative where explicitly defined by this and `unsafe.md`.

---

## § 9 Integer and address conversion

§ 9(1) Converting an arbitrary integer address to `RawPtr[T]` requires unsafe.

§ 9(2) Converting `RawPtr[T]` to a target-sized unsigned integer representation requires an explicit conversion under the applicable low-level rule.

§ 9(3) Integer/address conversion does not prove that the resulting address is mapped, live, aligned, initialized, accessible, owned, or valid for `T`.

§ 9(4) The conversion must preserve the selected target pointer width.

§ 9(5) A compile-time integer that cannot be represented by the target pointer representation must be rejected.

§ 9(6) Truncation of pointer bits is never implicit.

§ 9(7) Widening to an integer representation must preserve the complete pointer representation required by the target.

§ 9(8) Targets with non-flat or capability/address-space pointer representations may reject generic integer conversion or require a target-specific operation.

§ 9(9) The implementation must not assume compiler-host pointer width.

---

## § 10 RawPtr compatibility conversions

§ 10(1) Conversion between two `RawPtr` types is safe only where their canonical representation relationship is explicitly defined as representation-compatible and the conversion itself does not assert pointee validity.

§ 10(2) Representation compatibility does not grant permission to dereference the converted pointer as the target pointee type.

§ 10(3) Reinterpreting unrelated raw-pointer pointee types where target validity or alignment assumptions change requires unsafe.

§ 10(4) `RawPtr[void]` may participate in FFI-compatible raw address conversions according to FFI and target rules.

§ 10(5) Conversion does not transfer ownership of pointee storage.

§ 10(6) Conversion does not create a safe borrow.

---

## § 11 Raw-pointer read

§ 11(1) Canonical typed raw-pointer read is:

```sec
let value := unsafe pointer.Read()
```

for `pointer: RawPtr[T]`.

§ 11(2) `Read()` produces a value of type `T`.

§ 11(3) `Read()` does not create pointee ownership merely because a value is copied out.

§ 11(4) Before `Read()`, the programmer/trusted wrapper must ensure at least:

```text
pointer is non-null
pointer is aligned for T
address is readable in the active address space
storage is valid for at least one T
storage contains an initialized valid T representation
storage remains live for the complete read operation
the access does not violate ownership or borrow/alias obligations
target/platform access policy permits the read
```

§ 11(5) If `T` is not safely copy-readable from the underlying storage according to the ownership/representation rules, `Read()` must be rejected or require a more specialized operation.

§ 11(6) `RawPtr[void].Read()` is invalid.

§ 11(7) Ordinary `Read()` is distinct from `VolatileRead()`.

---

## § 12 Raw-pointer write

§ 12(1) Canonical typed raw-pointer write is:

```sec
unsafe pointer.Write(value)
```

for `pointer: RawPtr[T]`.

§ 12(2) Before `Write(value)`, the programmer/trusted wrapper must ensure at least:

```text
pointer is non-null
pointer is aligned for T
address is writable in the active address space
storage is valid for at least one T
the value representation is valid for the destination
the access satisfies required exclusive/mutable authority
no conflicting safe reference or foreign access violates the contract
overwriting does not bypass required destruction of a live value
target/platform access policy permits the write
```

§ 12(3) `Write()` does not grant ownership of the destination storage.

§ 12(4) `Write()` must not silently destroy a previously live non-trivial value unless the owning low-level API explicitly defines and proves replacement semantics.

§ 12(5) `RawPtr[void].Write(value)` is invalid.

§ 12(6) Ordinary `Write()` is distinct from `VolatileWrite()`.

---

## § 13 Volatile raw-pointer access

§ 13(1) Volatile raw-pointer access uses distinct compiler-known operations.

Canonical forms:

```sec
let value := unsafe pointer.VolatileRead()
```

```sec
unsafe pointer.VolatileWrite(value)
```

§ 13(2) Volatile operations are owned jointly by this raw-address boundary and `rules/platform/volatile.md`.

§ 13(3) `VolatileRead()` and `VolatileWrite()` do not convert `RawPtr[T]` into a volatile-qualified type.

§ 13(4) Volatile access does not create ownership.

§ 13(5) Volatile access does not establish synchronization.

§ 13(6) Volatile access does not establish hardware privilege, mapping authority, DMA ownership, cache coherence, or atomicity.

§ 13(7) `RawPtr[void]` direct volatile read/write is invalid without concrete compatible pointee interpretation.

§ 13(8) Volatile effect ordering and exact physical access width are governed by platform/volatile rules.

---

## § 14 Pointer arithmetic

§ 14(1) Raw-pointer arithmetic requires unsafe.

§ 14(2) Sec does not use ordinary numeric `+`, `-`, `++`, or `--` operators for raw-pointer arithmetic.

§ 14(3) Canonical compiler-known operations are:

```sec
pointer.Offset(elements)
pointer.AddBytes(bytes)
pointer.Difference(other)
```

§ 14(4) `Offset(elements: int)` computes an address offset measured in elements of `T`.

§ 14(5) `Offset()` is invalid for `RawPtr[void]`.

§ 14(6) `AddBytes(bytes: int)` is defined for `RawPtr[byte]`.

§ 14(7) `Difference(other: RawPtr[T]) int` requires compatible pointers with the same element type.

§ 14(8) `Difference()` reports an element difference where the operation is valid for the selected target/address domain.

§ 14(9) Pointer arithmetic does not create a valid safe reference.

§ 14(10) Pointer arithmetic does not by itself prove the result remains within one allocation, mapped region, hardware window, or valid object.

§ 14(11) The caller must ensure address calculation does not wrap into an invalid address.

§ 14(12) Later typed access must still satisfy alignment, lifetime, bounds, provenance, alias, and target-address obligations.

§ 14(13) Targets whose pointer/address representation does not support a requested arithmetic operation may reject it.

---

## § 15 Address overflow and range

§ 15(1) Raw address arithmetic must use selected-target pointer semantics.

§ 15(2) Compile-time provable address overflow or wraparound is rejected.

§ 15(3) Dynamic raw arithmetic that may leave a valid region remains an unsafe caller obligation unless a checked platform API provides recoverable validation.

§ 15(4) `unsafe` does not authorize the compiler to assume wraparound is harmless.

§ 15(5) Backend lowering must not use host arithmetic semantics when target pointer width/address space differs.

---

## § 16 Equality and ordering

§ 16(1) Compatible `RawPtr` values may be compared for equality.

§ 16(2) Raw-pointer equality compares raw address identity according to the selected target representation.

§ 16(3) Raw-pointer equality does not imply that either pointer is dereferenceable.

§ 16(4) Raw-pointer equality is distinct from safe-reference equality, which may include live storage identity/generation semantics.

§ 16(5) General raw-pointer ordering comparisons are not part of portable Sec 0.1 semantics.

§ 16(6) Ordering may be provided by a platform-specific API for an address domain where ordering is meaningful.

---

## § 17 Conversion from safe references

§ 17(1) Conversion from `ref T` or `ref mut T` to a raw pointer crosses from safe reference semantics into raw-address semantics.

§ 17(2) Such conversion requires unsafe unless a dedicated compiler-verified FFI/platform adapter is explicitly defined as safe.

§ 17(3) The resulting `RawPtr[T]` does not retain safe-reference guarantees after the originating reference or storage lifetime ends.

§ 17(4) Raw-pointer use must not be used to evade an active borrow conflict and then reconstruct a conflicting safe reference.

§ 17(5) A raw pointer derived from a safe reference does not keep the referent alive.

§ 17(6) The exact source spelling of this conversion remains owned by the future canonical conversion rule; this rewrite fixes the semantics, not a new conversion token/form.

---

## § 18 Conversion to safe references

§ 18(1) Constructing `ref T` from `RawPtr[T]` requires unsafe.

§ 18(2) The programmer/trusted wrapper must guarantee at least:

```text
non-null address
correct alignment
valid initialized T representation
readable storage
storage lifetime covers the complete reference lifetime
safe-reference provenance can be established for the storage
no conflicting mutable authority exists
foreign/hardware mutation does not violate shared-reference semantics
the reference will not escape beyond the proven storage lifetime
address-space and platform rules permit safe reference formation
```

§ 18(3) Constructing `ref mut T` additionally requires:

```text
writable storage
exclusive mutable authority for the complete borrow live range
no conflicting shared or mutable safe references
no unmodeled foreign/hardware mutation violating exclusivity
valid destruction/replacement semantics for mutation performed through the reference
```

§ 18(4) Unsafe context permits the programmer to accept obligations the compiler cannot fully prove; it does not remove obligations the compiler can prove false.

§ 18(5) A compiler-proven null, dead, unaligned, invalidly represented, or conflicting conversion must be rejected even inside unsafe.

§ 18(6) The exact Sec 0.1 source spelling for these conversions is not newly locked by this rulebook revision.

---

## § 19 Slice/view construction from raw parts

§ 19(1) Constructing a safe slice or bounded view from `RawPtr[T]` plus extent requires unsafe unless a specialized trusted wrapper provides a verified safe operation.

§ 19(2) Required obligations include:

```text
pointer valid for the complete extent
extent non-negative and representable
byte-size multiplication does not overflow
alignment valid for T
every readable element contains a valid initialized T
storage remains live for the complete slice/view lifetime
borrow/alias/mutability obligations are satisfied
the full range lies within one compatible storage or target region where required
```

§ 19(3) A zero-length slice does not permit arbitrary fabrication of a non-null safe scalar `ref T`.

§ 19(4) Safe slice/view construction must establish canonical provenance/lifetime facts required by `references.md`.

§ 19(5) No hidden allocation is introduced by slice/view construction.

---

## § 20 Uninitialized storage

§ 20(1) `RawPtr[T]` may address uninitialized storage.

§ 20(2) Reading uninitialized storage as ordinary `T` is invalid unless a dedicated low-level operation defines a valid uninitialized-memory workflow.

§ 20(3) Unsafe context does not make every bit pattern a valid `T`.

§ 20(4) Invalid representations may include:

```text
invalid enum discriminants
invalid union states
invalid bool/char/rune representations
violated named-type contracts
invalid pointer/reference representations
foreign-layout violations
ownership-state violations
```

§ 20(5) Construction from uninitialized memory into a safe value must establish every invariant required by the destination type.

---

## § 21 Ownership

§ 21(1) `RawPtr[T]` never owns pointee memory by default.

§ 21(2) Pointee ownership must be represented separately by a canonical ownership-bearing abstraction such as:

```text
allocator result/wrapper
foreign ownership contract
explicit resource type
owning buffer/container
platform mapping/resource owner
```

§ 21(3) The compiler must never free pointee memory solely because a `RawPtr[T]` value leaves scope.

§ 21(4) `RawPtr[T]` must never implicitly convert into an owning container, string, collection, buffer, mapping, or other owner.

§ 21(5) Adopting ownership from a raw pointer is a distinct unsafe operation with allocator/deallocator/lifetime proof obligations.

§ 21(6) Ownership adoption must identify the correct reclamation contract; numeric address alone is insufficient.

---

## § 22 Destruction

§ 22(1) Destroying a raw-pointer value is trivial with respect to the pointee.

§ 22(2) Raw-pointer scope exit performs no pointee destruction.

§ 22(3) Raw-pointer overwrite performs no pointee destruction.

§ 22(4) If a wrapper owns a foreign/resource allocation and also stores `RawPtr`, destruction belongs to the wrapper's ownership contract, not to `RawPtr`.

§ 22(5) A raw pointer to a Sec-owned value does not suppress that owner's ordinary destruction.

§ 22(6) Keeping a raw pointer after pointee destruction leaves a dangling raw address; it does not keep the pointee alive.

---

## § 23 Borrowing and aliases

§ 23(1) `RawPtr[T]` is not itself a Sec borrow.

§ 23(2) Copying raw pointers does not create shared-borrow authority.

§ 23(3) Raw pointers may exist while safe references exist when the applicable unsafe/FFI/platform contract permits it.

§ 23(4) Using raw access in a way that violates live safe-reference alias/exclusivity obligations is invalid.

§ 23(5) Converting raw pointers back into safe references must re-establish the complete safe borrow contract.

§ 23(6) Unsafe code must not use `RawPtr` as a loophole to manufacture two conflicting `ref mut` values.

§ 23(7) Known conflicting raw-to-safe conversion is rejected even inside unsafe.

---

## § 24 Lifetime

§ 24(1) A `RawPtr[T]` value may outlive the storage whose address it contains.

§ 24(2) That fact does not make later dereference valid.

§ 24(3) The compiler may retain known origin/lifetime facts for diagnostics and optimization when available.

§ 24(4) Absence of known lifetime metadata does not imply validity.

§ 24(5) Unsafe code is responsible for respecting lifetime obligations that cannot be proven statically.

§ 24(6) A raw pointer does not extend Arena generation, mapping lifetime, stack lifetime, object lifetime, or foreign retention period.

§ 24(7) A compiler-proven use after known storage end must be rejected even in unsafe code.

---

## § 25 Allocation

§ 25(1) Creating, copying, moving, comparing, or performing arithmetic on `RawPtr[T]` does not allocate.

§ 25(2) Raw-pointer dereference does not allocate merely because the operation is unsafe.

§ 25(3) Safe-reference or slice conversion from a raw pointer must not silently allocate to repair lifetime or provenance.

§ 25(4) Foreign allocator results remain foreign/raw allocation until wrapped by a canonical ownership-bearing Sec abstraction.

§ 25(5) Arena allocation is not represented merely by a `RawPtr[T]` return in safe Sec APIs when a safer bounded/reference abstraction is canonically required.

---

## § 26 FFI

§ 26(1) `RawPtr[T]` is the canonical raw-address boundary type for foreign APIs where safe Sec references cannot express the foreign contract.

§ 26(2) Nullable foreign pointers may use `RawPtr[T]`.

§ 26(3) A foreign declaration may accept or return `RawPtr[T]` without making the raw-pointer value itself an owner.

§ 26(4) FFI contracts must separately describe, where relevant:

```text
nullability
read/write permission
call-bounded borrowing
retention after return
ownership transfer
foreign deallocation
concurrent use
callback use
alignment
extent
ABI/address-space requirements
```

§ 26(5) Unknown foreign ownership or retention behavior is conservative.

§ 26(6) Passing a `RawPtr[T]` to foreign code does not by itself transfer ownership.

§ 26(7) Returning a `RawPtr[T]` from foreign code does not by itself transfer ownership to Sec.

§ 26(8) A safe wrapper may normalize a raw foreign pointer into `Option`, `Result`, safe reference, slice, or owning wrapper only after establishing the required contract.

---

## § 27 Fixed hardware addresses

§ 27(1) Stable known hardware addresses should use canonical fixed-address declarations where platform knowledge can verify them.

Example:

```sec
@address(Peripheral.GPIOA)
let mut GPIOA: GPIORegisters
```

§ 27(2) `@address` is not equivalent to integer-to-`RawPtr` conversion.

§ 27(3) Accepted `@address` declarations receive target/platform validation that arbitrary `RawPtr` values do not possess automatically.

§ 27(4) A raw numeric address may be converted to `RawPtr[T]` under unsafe low-level rules when an application truly requires raw addressing.

§ 27(5) Possessing such a raw pointer does not grant:

```text
hardware privilege
mapping authority
security-domain authority
resource ownership
device liveness
canonical peripheral identity
```

§ 27(6) Hardware access remains subject to selected target/address-space rules.

---

## § 28 Runtime-discovered hardware mappings

§ 28(1) Runtime-discovered hardware/device mappings should normally use the checked mapping/resource model from `hardware-register-access.md`.

§ 28(2) A mapping owner and a raw pointer into its mapped region are distinct values with different semantics.

§ 28(3) A `RawPtr[T]` into a mapping does not keep the mapping alive.

§ 28(4) Mapping destruction/remap may leave prior raw pointers dangling.

§ 28(5) Safe typed register views require mapping/lifetime proof that a bare `RawPtr[T]` does not provide.

§ 28(6) Low-level platform libraries may use raw pointers internally under unsafe proof obligations to implement safe mapping/view APIs.

---

## § 29 Interrupt execution

§ 29(1) `RawPtr[T]` operations in ISR execution obey ordinary raw-pointer and interrupt rules.

§ 29(2) `unsafe` does not waive `@isr` or `@interruptSafe` requirements.

§ 29(3) Raw-pointer operations reachable from an ISR must still satisfy `noPanic`, `noAlloc`, `noBlock`, bounded-work, synchronization, and permitted-effect requirements.

§ 29(4) Raw hardware access does not become ISR-safe merely because it uses `RawPtr`.

§ 29(5) Volatile access is not synchronization between ISR and non-ISR execution.

§ 29(6) Any compiler-known raw-pointer helper used from an ISR must carry its complete effect summary.

---

## § 30 Address spaces and target dependence

§ 30(1) `RawPtr[T]` lowering uses the selected target's canonical pointer/address-space representation.

§ 30(2) The compiler must not assume all targets have one flat address space.

§ 30(3) Pointer width, address space, alignment, and representation are target facts.

§ 30(4) Raw-pointer compatibility/conversion rules must preserve address-space correctness.

§ 30(5) A target may reject conversion between incompatible address spaces.

§ 30(6) A target may require capability/tag/provenance representation beyond an integer address.

§ 30(7) Generic raw-pointer semantics remain source-level address semantics; target representation may be richer.

---

## § 31 Effects

§ 31(1) Possessing, copying, moving, or comparing a raw pointer has no dereference effect merely because the type is `RawPtr`.

§ 31(2) `Read()` and `Write()` carry memory-access effects appropriate to the storage domain.

§ 31(3) `VolatileRead()` and `VolatileWrite()` carry canonical volatile effects.

§ 31(4) Hardware/foreign raw accesses may additionally carry effects such as:

```text
MayIO
MayMutateExternalState
MayUseNondeterministicInput
MayAccessVolatile
```

§ 31(5) Unsafe context does not remove or suppress effects.

§ 31(6) Compiler-known raw-pointer operations must expose stable effect identities to analysis/tooling.

---

## § 32 Semantic IR

§ 32(1) Semantic IR must distinguish raw pointers from safe references.

§ 32(2) Semantic IR must not attach ownership, non-nullness, bounds, lifetime, or generation guarantees merely because a value has type `RawPtr[T]`.

§ 32(3) Semantic IR must preserve, where required:

```text
pointee type
address space
selected pointer representation
raw-pointer operation identity
unsafe/trust provenance
known nullability facts
known origin/range facts
known alignment facts
volatile access identity
effect sites
target/platform access contract
```

§ 32(4) Typed `Read`, `Write`, volatile operations, and arithmetic must remain distinguishable until equivalent lower-level semantics are materialized.

§ 32(5) Raw-to-safe conversion must explicitly establish the safe-reference facts required by `references.md`.

§ 32(6) Semantic IR verification must reject contradictory raw-pointer facts.

---

## § 33 Lowering

§ 33(1) Lowering must preserve target pointer width/address-space semantics.

§ 33(2) Lowering must not treat `RawPtr[T]` as an owner or safe reference.

§ 33(3) Typed read/write lowering must use target-correct size and alignment.

§ 33(4) Volatile raw-pointer operations must preserve volatile ordering/access constraints from `volatile.md`.

§ 33(5) Pointer arithmetic must use target layout for element-size scaling.

§ 33(6) Address arithmetic must not silently truncate.

§ 33(7) Backend alias/noalias metadata must not be stronger than proven Sec facts.

§ 33(8) A raw pointer must not automatically receive `nonnull`, `dereferenceable`, safe-lifetime, unique-alias, or ownership metadata.

§ 33(9) Raw pointer to integer lowering must preserve the complete representable target address where the conversion is supported.

§ 33(10) Backend inability to model a required target pointer/address-space operation is a compile/lowering error, not permission to change semantics.

---

## § 34 Diagnostics

§ 34(1) Raw-pointer diagnostics must follow the mentor-compiler principle.

§ 34(2) Diagnostics should distinguish at least:

```text
missing unsafe context
invalid raw-pointer type
null dereference known statically
known dangling/lifetime violation
known alignment violation
known alias conflict
invalid pointer arithmetic
unsupported address-space conversion
target-width overflow/truncation
uninitialized representation read
invalid raw-to-safe conversion
unsupported RawPtr[void] operation
hardware access contract violation
```

§ 34(3) A diagnostic should identify the exact unsafe operation and the proof obligation that is missing or contradicted.

§ 34(4) Diagnostics must not imply that adding `unsafe` fixes a compiler-proven invalid condition.

Example intent:

```text
error: this raw-pointer read is known to use a null address

`unsafe` can accept obligations that the compiler cannot prove, but it does not
make a proven null dereference valid.

help: test the raw pointer for null before reading it
```

---

## § 35 LSP and tooling

§ 35(1) LSP completion must expose canonical compiler-known `RawPtr[T]` members.

§ 35(2) Unsafe raw-pointer members must be clearly marked as unsafe.

§ 35(3) Hover should expose operation identity, pointee type, unsafe requirement, and relevant effects.

§ 35(4) Where known, tooling may expose address space, nullability, provenance/range, alignment, volatile access contract, and lifetime facts.

§ 35(5) Tooling must not invent safety facts merely because code occurs inside an unsafe block.

§ 35(6) Compiler and LSP must consume the same compiler-known raw-pointer registry.

---

## § 36 Formatter

§ 36(1) Formatter preserves canonical generic type syntax:

```sec
RawPtr[T]
```

§ 36(2) Formatter preserves canonical unsafe forms:

```sec
let value := unsafe pointer.Read()
unsafe pointer.Write(value)
```

§ 36(3) Formatter preserves compiler-known member spellings:

```sec
pointer.Offset(elements)
pointer.AddBytes(bytes)
pointer.Difference(other)
pointer.VolatileRead()
pointer.VolatileWrite(value)
```

§ 36(4) Formatter must not invent C-style pointer syntax.

§ 36(5) Future explicit raw/reference conversion syntax must be formatted only after its grammar is canonically locked.

---

## § 37 Required test families

### § 37.1 Type/value tests

§ 37.1(1) Required tests include:

```text
RawPtr[T] type construction
RawPtr[void]
copy raw pointer
move raw pointer
store raw pointer in aggregate
raw-pointer scope exit does not free pointee
raw pointer may be null
```

### § 37.2 Unsafe gating

§ 37.2(1) Required tests include:

```text
Read requires unsafe
Write requires unsafe
VolatileRead requires unsafe
VolatileWrite requires unsafe
Offset requires unsafe
AddBytes requires unsafe
Difference requires unsafe
integer-to-RawPtr requires unsafe
raw-to-safe reference construction requires unsafe
slice-from-raw-parts requires unsafe
unsafe fn body still requires explicit unsafe operation
```

### § 37.3 Proven invalid operations

§ 37.3(1) Required tests include:

```text
known null dereference rejected inside unsafe
known dangling use rejected
known invalid alignment rejected
known alias conflict rejected
known uninitialized read rejected
known pointer-width overflow rejected
RawPtr[void].Read rejected
RawPtr[void].Write rejected
```

### § 37.4 Arithmetic

§ 37.4(1) Required tests include:

```text
Offset scales by sizeof(T)
Offset rejects RawPtr[void]
AddBytes requires RawPtr[byte]
Difference requires compatible pointee type
address overflow/wrap rejected where provable
target pointer width used instead of compiler-host width
unsupported address-space arithmetic rejected
```

### § 37.5 Ownership/lifetime

§ 37.5(1) Required tests include:

```text
raw-pointer copy does not copy pointee
raw-pointer move does not move pointee
raw-pointer destruction does not destroy pointee
raw pointer does not keep Arena storage alive
raw pointer does not keep mapping alive
raw pointer does not keep stack local alive
raw-to-safe conversion must establish reference lifetime
raw pointer cannot be used to manufacture conflicting ref mut values
```

### § 37.6 FFI/platform

§ 37.6(1) Required tests include:

```text
nullable FFI raw pointer accepted
foreign RawPtr return does not imply ownership
foreign RawPtr parameter does not imply ownership transfer
@address is distinct from raw numeric pointer construction
runtime mapping lifetime not extended by RawPtr
ordinary Read/Write distinct from VolatileRead/VolatileWrite
volatile access is not synchronization
ISR raw-pointer helper effects participate in interrupt analysis
```

### § 37.7 IR/lowering/tooling

§ 37.7(1) Required tests include:

```text
Semantic IR preserves RawPtr distinct from safe ref
Semantic IR preserves address space
RawPtr receives no automatic nonnull/dereferenceable/ownership facts
read/write lower with target-correct size/alignment
volatile access remains volatile
pointer arithmetic uses target layout
LSP completion exposes canonical members
LSP marks unsafe operations
compiler/LSP agree on raw-pointer diagnostics
```

---

## § 38 Completion criteria

§ 38(1) Frontend raw-pointer support is complete when `RawPtr[T]`, null handling, canonical raw operations, target-aware conversions, unsafe gating, and known-invalid operation rejection are implemented for all Sec 0.1 forms.

§ 38(2) Raw/reference integration is complete when safe-reference construction from raw storage establishes complete provenance, lifetime, borrow, non-nullness, initialization, alignment, address-space, and platform facts.

§ 38(3) FFI integration is complete when nullability, retention, ownership, extent, alignment, concurrency, and deallocation contracts interact canonically with raw pointers.

§ 38(4) Platform integration is complete when fixed addresses, runtime mappings, volatile operations, address spaces, hardware regions, and ISR effects consume the same raw-pointer facts.

§ 38(5) Semantic IR support is complete when every maintained raw-pointer operation carries sufficient target/address/effect facts without being confused with ownership or safe references.

§ 38(6) Lowering support is complete when all maintained targets preserve pointer width, address spaces, alignment, arithmetic, volatile behavior, and raw semantics without stronger unproven alias/lifetime metadata.

§ 38(7) Tooling support is complete when compiler, LSP, formatter, diagnostics, and `sec analyse` consume the same canonical raw-pointer registry and facts.

§ 38(8) Raw pointers must not be marked fully implemented merely because `RawPtr[T]` parses and the frontend recognizes `Offset`, `AddBytes`, and `Difference`.

---

## § 39 Core summary

§ 39(1) `RawPtr[T]` is an unchecked raw address value, not a safe reference.

§ 39(2) `RawPtr[T]` is copyable and non-owning.

§ 39(3) Null is part of the raw-pointer value domain.

§ 39(4) Storing, copying, moving, returning, passing, null-testing, and compatible equality do not require unsafe merely because the value is a raw pointer.

§ 39(5) Reading, writing, volatile access, arithmetic, integer-address construction, raw-to-safe reference construction, slice construction, ownership adoption, and unrelated reinterpretation require unsafe unless a specialized verified operation says otherwise.

§ 39(6) Canonical raw-pointer operations include:

```sec
pointer.Read()
pointer.Write(value)
pointer.VolatileRead()
pointer.VolatileWrite(value)
pointer.Offset(elements)
pointer.AddBytes(bytes)
pointer.Difference(other)
```

§ 39(7) `unsafe` accepts proof obligations; it does not make compiler-proven invalid operations valid.

§ 39(8) Raw pointers do not extend pointee lifetime, own pointee storage, create borrows, provide bounds, guarantee alignment, or establish hardware privilege.

§ 39(9) Safe-reference construction from raw storage must re-establish the complete safe-reference contract.

§ 39(10) Exact source spelling for `RawPtr ↔ ref/ref mut` conversion remains intentionally outside this rewrite until canonically locked elsewhere.

§ 39(11) Target pointer representation and address spaces are target facts; the compiler must never substitute compiler-host assumptions.

§ 39(12) Fixed-address declarations and checked hardware mappings are preferred when the platform can provide stronger verified semantics than an arbitrary raw address.
