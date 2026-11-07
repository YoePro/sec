# Parameter Usage Analysis

- **Status:** Normative
- **Created:** 2026-08-08
- **Last updated:** 2026-11-07 (stable chapter, subsection, and paragraph numbering)
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/analysis/parameter_usage_analysis.md`
- **Replaces:** Earlier unversioned revision at the same canonical path
- **Repository baseline reviewed:** `28780c01d63c3fc9eec2321ef67e3dd69962f043`
- **Implementation governance:** `governance/analysis.yaml` (`sema.parameter-usage-analysis`)
- **Related rulebooks:** `rules/declarations/functions.md`, `rules/memory/copy_move.md`, `rules/memory/ownership.md`, `rules/memory/borrowing.md`, `rules/memory/reference_model.md`, `rules/memory/storage.md`, `rules/memory/layout.md`, `rules/analysis/escape_analysis.md`, `rules/analysis/closure_analysis.md`, `rules/analysis/call_graph.md`, `rules/analysis/effect_analysis.md`, `rules/platform/ffi.md`, `rules/tooling/diagnostics.md`, `rules/tooling/lsp.md`, `rules/compiler/incremental_compilation.md`

---

## § 1 Status and authority

**§ 1(1)** This is the normative compiler-analysis rulebook for Sec 0.1 parameter usage
analysis.

**§ 1(2)** This rulebook defines the semantic purpose, demand model, transfer rules,
interprocedural propagation, FFI contract boundaries, narrowing-candidate model,
recommendation policy, tooling integration, summary requirements, diagnostics,
and completion criteria for Sec parameter usage analysis.

**§ 1(3)** Mutable implementation status belongs to `sema.parameter-usage-analysis` in
`governance/analysis.yaml`.

**§ 1(4)** Source-level parameter ownership syntax and copy/move legality remain owned by
`rules/declarations/functions.md` and `rules/memory/copy_move.md`.

---

## § 2 Purpose

**§ 2(1)** Parameter usage analysis determines the minimum semantic capabilities that a
function implementation requires from each parameter and receiver.

**§ 2(2)** The analysis answers questions such as:

```text
Does the callee require ownership?
Does it only require a borrow?
Does it mutate elements or collection structure?
Does it require an exact fixed-array extent?
Does it only require sequence access?
Does it require a stable address?
Does it retain the argument or a dependency on it?
Does a foreign call impose stronger storage or representation requirements?
```

**§ 2(3)** The result may reveal that the declared source-level parameter contract is
stronger than the implementation requires.

**§ 2(4)** Examples include:

- a large by-value struct whose implementation only performs call-local reads;
- a fixed array whose implementation only requires a read-only sequence view;
- an owning collection whose implementation only requires element access;
- a mutable parameter whose implementation never performs mutation;
- an FFI wrapper that requires an owning fixed array even though the foreign
  operation only requires a call-local pointer and length;
- a chain of functions that all forward an unnecessarily strong parameter
  contract.

**§ 2(5)** Parameter usage analysis is primarily an API-design and efficiency analysis.
It does not redefine the validity of a stronger source-level parameter contract.

---

## § 3 Normative role

**§ 3(1)** This rulebook is normative for:

- the concept of `ParameterDemand`;
- the distinction between declared capability and required capability;
- the demand dimensions that the compiler must be able to represent;
- the transfer rules by which parameter uses raise demand;
- interprocedural propagation of parameter demand;
- conservative control-flow joins and recursive fixed points;
- the treatment of function-value calls and callable contracts;
- FFI as an explicit contract boundary;
- dimension-specific unknown demand;
- candidate parameter narrowing;
- separation between semantic demand and recommendation policy;
- Interactive, Standard, and Deep analysis behavior as it applies to parameter
  usage analysis;
- separate-compilation summary requirements;
- diagnostic/advisory requirements;
- required test classes;
- completion criteria for Sec 0.1 parameter usage analysis.

**§ 3(2)** This rulebook does not redefine:

- source-level parameter syntax;
- reference syntax;
- array or slice syntax;
- ownership;
- copy or move legality;
- borrow legality;
- object lifetime;
- storage lifetime;
- escape semantics;
- callable semantics;
- FFI declaration syntax;
- physical ABI argument passing;
- physical stack placement;
- physical closure representation;
- source-level overload resolution.

**§ 3(3)** Those semantics remain owned by their respective normative rulebooks.

---

## § 4 Ownership-transfer syntax boundary

**§ 4(1)** Parameter usage analysis consumes resolved ownership-transfer facts.

**§ 4(2)** It does not infer consumption from obsolete source spellings or from type
copyability alone.

**§ 4(3)** The canonical Sec 0.1 source rules are:

```sec
fn Consume(-> value: Buffer) void {
    ...
}

Consume(<-buffer)
```

**§ 4(4)** A consuming parameter is declared with `->`.

**§ 4(5)** Passing an existing reusable source Place to a consuming parameter requires
`<-` at the call site.

**§ 4(6)** A fresh temporary may be passed directly:

```sec
Consume(CreateBuffer())
```

**§ 4(7)** The call-site marker is not required for a fresh temporary because there is no
reusable caller binding whose availability must be explicitly surrendered.

**§ 4(8)** A normal by-value parameter is not consuming merely because its type is
move-only.

**§ 4(9)** Parameter usage analysis therefore derives consumption demand from the resolved
semantic operation and callable contract, not from spelling heuristics or from
the physical ABI.

**§ 4(10)** Return is a separate ownership boundary.

**§ 4(11)** For a non-reference return:

```sec
return value
```

**§ 4(12)** ownership of an owned local may transfer to the caller without a mandatory
`<-` marker.

**§ 4(13)** The optional explicit form:

```sec
return <-value
```

**§ 4(14)** has the same transfer meaning when legal.

**§ 4(15)** Parameter-demand analysis must therefore record the semantic return transfer,
not test for presence of a call-site move marker.

---

## § 5 Core rule

**§ 5(1)** Parameter usage analysis determines the weakest semantic parameter contract that
is sufficient for every reachable use in the analyzed function.

**§ 5(2)** The analysis does not change the source declaration.

**§ 5(3)** For example:

```sec
fn Sum(values: int[10000]) int {
    let mut total := 0

    for i in 0..<values.len {
        total += values[i]
    }

    return total
}
```

**§ 5(4)** The declared parameter requires a fixed array with exact extent `10000`.

**§ 5(5)** The implementation may only require:

```text
read access
call-local lifetime
random-access sequence semantics
no ownership
no structural mutation
no exact extent
```

**§ 5(6)** That difference creates a candidate API narrowing.

**§ 5(7)** It does not authorize the compiler to silently change the function type.

---

## § 6 Source semantics and ABI are distinct

**§ 6(1)** Source-level parameter semantics are not the same as physical argument passing.

**§ 6(2)** A source declaration such as:

```sec
fn Inspect(value: LargeStruct) void {
    Use(value.Field)
}
```

**§ 6(3)** remains a by-value source parameter even if ABI lowering passes the physical
value indirectly through a hidden address.

**§ 6(4)** Conversely, parameter usage analysis may conclude that a shared reference would
be a sufficient and potentially better source-level API even when the current
ABI already avoids a physical copy.

**§ 6(5)** Therefore:

```text
Source parameter semantics
    !=
Physical ABI argument passing
```

**§ 6(6)** Parameter usage analysis owns the first question.

**§ 6(7)** ABI lowering owns the second.

---

## § 7 Declared capability and required capability

**§ 7(1)** Each analyzed parameter has two conceptually separate capability sets:

```text
DeclaredCapability
RequiredCapability
```

**§ 7(2)** `DeclaredCapability` is derived from the source-level parameter type and its
normative contracts.

**§ 7(3)** `RequiredCapability` is derived from the reachable implementation and applicable
callee/foreign contracts.

**§ 7(4)** A narrowing opportunity exists when:

```text
DeclaredCapability
    is strictly stronger than
RequiredCapability
```

**§ 7(5)** The existence of such a relation does not prove that the programmer intended a
weaker public API.

**§ 7(6)** The implementation demand and API intent remain distinct concepts.

---

## § 8 ParameterDemand

**§ 8(1)** Parameter demand is multidimensional.

**§ 8(2)** The primary result must not be reduced to a single classification such as:

```text
ShouldBeRef = true
```

**§ 8(3)** Conceptually, the analysis must be able to represent:

```text
ParameterDemand {
    Access
    Mutation
    Ownership
    Lifetime
    Identity
    Shape
    Storage
    Representation
    Precision
}
```

**§ 8(4)** The exact compiler data structure is implementation-defined.

**§ 8(5)** The semantic dimensions are normative.

---

## § 9 Access demand

**§ 9(1)** Canonical access demand includes at least:

```text
Unused
Read
Write
Unknown
```

### § 9.1 Unused

**§ 9.1(1)** The implementation has no reachable semantic use of the parameter.

**§ 9.1(2)** Unused-parameter diagnostics may be owned elsewhere, but the fact remains a
valid parameter-demand result.

### § 9.2 Read

**§ 9.2(1)** The implementation observes the parameter or one of its components without
requiring mutation.

**§ 9.2(2)** Examples include:

```sec
let name := customer.Name
let value := items[index]
if buffer.len == 0 { ... }
```

**§ 9.2(3)** Read demand does not imply ownership.

### § 9.3 Write

**§ 9.3(1)** The implementation requires mutation of the parameter or one of its components.

**§ 9.3(2)** Write demand does not by itself imply ownership.

**§ 9.3(3)** A mutable borrow may be sufficient.

---

## § 10 Mutation demand

**§ 10(1)** Collection and aggregate mutation must not be collapsed into one boolean.

**§ 10(2)** Canonical mutation facts include at least:

```text
NoMutation
ElementOrFieldMutation
StructuralMutation
UnknownMutation
```

### § 10.1 Element or field mutation

**§ 10.1(1)** Examples:

```sec
customer.Name = newName
values[index] = 0
```

**§ 10.1(2)** Such uses may be satisfiable through a mutable reference or mutable sequence
view without transferring ownership.

### § 10.2 Structural mutation

**§ 10.2(1)** Structural mutation changes collection structure or backing relation.

**§ 10.2(2)** Examples conceptually include:

```text
Append
Grow
Resize
Remove
RebindBackingStorage
```

**§ 10.2(3)** A mutable element view is not automatically sufficient for structural mutation.

**§ 10.2(4)** The required candidate contract must expose the structural capability actually
used.

---

## § 11 Ownership demand

**§ 11(1)** Canonical ownership demand includes at least:

```text
BorrowSufficient
OwnershipRequired
ConsumptionRequired
UnknownOwnership
```

### § 11.1 BorrowSufficient

**§ 11.1(1)** The function does not require ownership of the value.

**§ 11.1(2)** The required borrow may be shared or mutable depending on mutation demand.

### § 11.2 OwnershipRequired

**§ 11.2(1)** The function requires an owned value to satisfy reachable semantics, for
example because the value must become owned state somewhere else.

### § 11.3 ConsumptionRequired

**§ 11.3(1)** The function must be able to consume or move the parameter or a relevant owned
subobject.

**§ 11.3(2)** Example:

```sec
fn Enqueue(-> job: Job) void {
    queue.Add(<-job)
}
```

**§ 11.3(3)** The parameter itself is consuming.

**§ 11.3(4)** Inside the function, `job` is an owned named Place. Forwarding that existing
Place into another consuming parameter therefore uses `<-` at that call site.

**§ 11.3(5)** The analysis records `ConsumptionRequired` from the resolved ownership transfer,
not from the textual presence of `<-` alone.

**§ 11.3(6)** A borrowed parameter is not semantically equivalent.

---

## § 12 Current copy is not proof of required copy

**§ 12(1)** A parameter may currently be copied because its declaration is by-value.

**§ 12(2)** That does not prove that an independent copy is required by the implementation.

**§ 12(3)** Example:

```sec
fn Inspect(value: LargeCopyableStruct) int {
    return value.Count
}
```

**§ 12(4)** The implementation may require only a shared borrow even if ordinary source
parameter passing currently creates value semantics.

**§ 12(5)** The analysis must distinguish:

```text
what the current declaration causes
```

**§ 12(6)** from:

```text
what the implementation requires
```

**§ 12(7)** A semantic copy requirement exists only when a distinct copied value is itself
required by reachable semantics.

---

## § 13 Lifetime and retention demand

**§ 13(1)** Parameter usage analysis consumes escape and lifetime facts.

**§ 13(2)** It does not implement a second escape analysis.

**§ 13(3)** Canonical lifetime/retention demand includes at least:

```text
CallOnly
Returned
Retained
CrossTask
CrossThread
ForeignRetention
UnknownLifetime
```

### § 13.1 CallOnly

**§ 13.1(1)** No dependency on the parameter remains usable outside the dynamic invocation
except through ordinary value results that no longer depend on the parameter.

**§ 13.1(2)** Call-only parameters are strong candidates for borrowed APIs when other demand
dimensions permit it.

### § 13.2 Returned

**§ 13.2(1)** A returned result depends on the parameter.

**§ 13.2(2)** Examples include returned references or views into caller-provided storage.

**§ 13.2(3)** A borrowed parameter may still be appropriate, but its return-lifetime relation
must remain valid.

### § 13.3 Retained

**§ 13.3(1)** The function or a callee may retain a dependency after the call returns.

### § 13.4 CrossTask and CrossThread

**§ 13.4(1)** A dependency crosses into another execution context.

**§ 13.4(2)** Ownership, lifetime, storage, and synchronization requirements remain separate
and are consumed from their respective analyses.

### § 13.5 ForeignRetention

**§ 13.5(1)** Foreign code may retain the value, address, pointer, callback, view, or related
dependency according to an explicit FFI contract.

---

## § 14 Identity demand

**§ 14(1)** Parameter usage analysis distinguishes value observation from address/identity
requirements.

**§ 14(2)** Canonical identity demand includes at least:

```text
ValueOnly
AddressRequired
StableIdentityRequired
UnknownIdentity
```

### § 14.1 ValueOnly

**§ 14.1(1)** The function only needs semantic value access.

### § 14.2 AddressRequired

**§ 14.2(1)** The implementation forms or forwards a safe reference, raw pointer, address, or
other address-bearing capability.

**§ 14.2(2)** Address requirement alone does not imply long-lived stable identity.

### § 14.3 StableIdentityRequired

**§ 14.3(1)** The same object/storage identity must remain valid for a longer semantic
interval, for example because a reference is retained, a foreign API requires a
stable address, or another contract explicitly depends on identity.

**§ 14.3(2)** Stable identity is derived from reference, escape, storage, FFI, and callable
contracts rather than from syntax alone.

---

## § 15 Shape demand

**§ 15(1)** Collection and fixed-array parameters require independent shape facts.

**§ 15(2)** The analysis must be able to represent at least:

```text
WholeValue
Sequence
ContiguousSequence
RandomAccessSequence
ExactExtent
MinimumExtent
KnownRange
UnknownShape
```

**§ 15(3)** These facts may be combined.

**§ 15(4)** They are not required to form one exclusive enum.

---

## § 16 Sequence demand

**§ 16(1)** Iteration may require only sequence semantics.

**§ 16(2)** Example:

```sec
for value in values {
    Process(value)
}
```

**§ 16(3)** This does not automatically require random access or exact extent.

---

## § 17 Random-access demand

**§ 17(1)** Indexing requires random access when the relevant collection/view abstraction
uses random access semantics.

**§ 17(2)** Example:

```sec
let value := values[index]
```

**§ 17(3)** This does not automatically require ownership or exact fixed-array extent.

---

## § 18 Contiguous-sequence demand

**§ 18(1)** Some operations require contiguous storage.

**§ 18(2)** Examples may include:

- foreign pointer-plus-length calls;
- compiler-known byte operations;
- target-specific operations requiring contiguous memory.

**§ 18(3)** Contiguity is a stronger shape/storage capability than generic sequence access.

---

## § 19 Exact extent and length observation are distinct

**§ 19(1)** Observing `len` does not by itself mean that the declared exact fixed-array
extent is required.

**§ 19(2)** Example:

```sec
if values.len == 16 {
    ...
}
```

**§ 19(3)** This observes length.

**§ 19(4)** A genuine exact-extent requirement exists when the semantic contract or
reachable implementation requires exactly that extent.

**§ 19(5)** The analysis must not infer:

```text
Declared T[N]
    =>
ExactExtent(N) required
```

**§ 19(6)** without a semantic reason.

---

## § 20 Fixed-index access establishes minimum extent

**§ 20(1)** A fixed index may establish a minimum extent without requiring the full declared
extent.

**§ 20(2)** Example:

```sec
return values[7]
```

**§ 20(3)** conceptually implies:

```text
MinimumExtent >= 8
```

**§ 20(4)** not:

```text
ExactExtent = declared N
```

**§ 20(5)** The compiler may use this precision in Deep analysis even if no source-level
minimum-extent contract exists in Sec 0.1.

---

## § 21 Partial-range demand

**§ 21(1)** The analysis may retain known range usage.

**§ 21(2)** Example:

```sec
fn Header(packet: byte[4096]) Header {
    return ParseHeader(packet[0..<64])
}
```

**§ 21(3)** may yield:

```text
KnownRange = [0, 64)
```

**§ 21(4)** This fact may support API-design advisories or call-site slicing analysis.

**§ 21(5)** It does not automatically prove that `byte[64]` is the intended public API.

---

## § 22 Storage demand

**§ 22(1)** Parameter demand may include storage requirements obtained from canonical
storage facts and callee contracts.

**§ 22(2)** The analysis must be able to preserve at least:

```text
NoSpecialStorage
Contiguous
StableAddress
Aligned
Pinned
SpecificMemorySpace
UnknownStorage
```

**§ 22(3)** These may be combined.

**§ 22(4)** A generic borrowed view must not be recommended when a required storage
property would be lost.

---

## § 23 Representation demand

**§ 23(1)** Some operations require a particular physical representation or explicit
layout contract.

**§ 23(2)** Examples include:

```text
FFI struct layout
explicit packing
explicit endian representation
hardware-defined representation
compiler-known representation contract
```

**§ 23(3)** Representation demand is distinct from ordinary value, ownership, and shape
requirements.

**§ 23(4)** The analysis consumes `ResolvedLayout` and relevant FFI/hardware contracts.
Source-declared struct layout modifiers (MD-019: `pack`, `minAlign`,
`byteOffset`) may block narrowing advice; they are never evidence that a
prospective view or reference satisfies a foreign alignment or representation
contract unless that is proven.

**§ 23(5)** It must not infer representation requirements merely from type size.

---

## § 24 Operation-to-demand transfer

**§ 24(1)** Every reachable parameter use contributes the minimum semantic requirements of
that use.

**§ 24(2)** The total `ParameterDemand` is the conservative join of all reachable uses.

**§ 24(3)** The analysis must derive demand from semantic operations rather than from
superficial syntax patterns alone.

---

## § 25 Field reads

**§ 25(1)** Example:

```sec
let name := customer.Name
```

**§ 25(2)** normally contributes:

```text
Access = Read
```

**§ 25(3)** It does not automatically contribute:

```text
OwnershipRequired
AddressRequired
ExactExtent
```

---

## § 26 Field mutation

**§ 26(1)** Example:

```sec
customer.Name = newName
```

**§ 26(2)** contributes:

```text
Access = Write
Mutation = ElementOrFieldMutation
```

**§ 26(3)** An ordinary aggregate may therefore be compatible with a mutable-reference
candidate if all other demands permit it.

---

## § 27 Collection element read

**§ 27(1)** Example:

```sec
let x := values[i]
```

**§ 27(2)** normally contributes:

```text
Access = Read
Shape = RandomAccessSequence
```

**§ 27(3)** It does not by itself require exact extent, ownership, or structural mutation.

---

## § 28 Collection element mutation

**§ 28(1)** Example:

```sec
values[i] = 0
```

**§ 28(2)** normally contributes:

```text
Access = Write
Mutation = ElementOrFieldMutation
Shape = RandomAccessSequence
```

**§ 28(3)** It does not by itself require structural mutation or ownership.

---

## § 29 Structural collection operations

**§ 29(1)** A structural collection operation raises structural mutation demand.

**§ 29(2)** A candidate borrowed view is valid only if the candidate abstraction exposes the
required structural behavior.

**§ 29(3)** The analysis must not replace an owning/growable parameter with a fixed view
when the callee performs operations such as grow, resize, append, or backing
rebind.

---

## § 30 Taking a safe reference

**§ 30(1)** Example:

```sec
let r := ref value
```

**§ 30(2)** contributes at least:

```text
Identity = AddressRequired
```

**§ 30(3)** and the applicable lifetime/borrow dependency.

**§ 30(4)** The lifetime requirement depends on how the reference is subsequently used.

---

## § 31 Raw pointer/address formation

**§ 31(1)** Creating or forwarding a raw pointer similarly establishes address demand and
applicable unsafe/storage/FFI requirements.

**§ 31(2)** The analysis must not later recommend a candidate parameter type from which the
required pointer/address semantics cannot be formed safely and legally.

---

## § 32 Returning a parameter by value

**§ 32(1)** Example:

```sec
return value
```

**§ 32(2)** The resolved return operation may require ownership, copy, or transfer
capability according to ordinary Sec value semantics.

**§ 32(3)** A non-reference return is intrinsically a result-value ownership boundary.
Therefore `return value` may transfer an owned local even though `<-` is not
mandatory at the return boundary.

**§ 32(4)** The analysis must use the resolved return operation rather than treating absence
of `<-` as evidence that ownership is not transferred.

**§ 32(5)** The explicit equivalent form is permitted but not required at the return
boundary:

```sec
return <-value
```

**§ 32(6)** A read-only function body does not automatically imply that a borrow is
sufficient when the parameter value itself is returned or transferred.

---

## § 33 Returning a borrow/view

**§ 33(1)** Example:

```sec
return ref value.Field
```

**§ 33(2)** may yield:

```text
Lifetime = Returned
Identity = AddressRequired
```

**§ 33(3)** A borrowed source parameter may remain sufficient if lifetime rules prove the
returned dependency valid.

**§ 33(4)** Escape and lifetime analysis own that proof.

---

## § 34 Retaining/storing a parameter

**§ 34(1)** Storing a value into longer-lived state may raise ownership demand.

**§ 34(2)** Storing a borrow, view, address, callback, or other dependency may raise
retention demand.

**§ 34(3)** The specific result depends on the destination and applicable ownership/escape
contracts.

**§ 34(4)** The parameter usage analysis consumes those facts rather than inferring
retention from container syntax alone.

---

## § 35 Move use

**§ 35(1)** A reachable move/consume use contributes:

```text
Ownership = ConsumptionRequired
```

**§ 35(2)** for the relevant Place or subobject.

**§ 35(3)** Example:

```sec
fn Consume(-> item: Item) void {
    ...
}

Consume(<-value)
```

**§ 35(4)** The resolved call transfers ownership of the existing named Place `value` into
the consuming parameter.

**§ 35(5)** That use contributes:

```text
Ownership = ConsumptionRequired
```

**§ 35(6)** A borrow-only candidate cannot satisfy this demand.

**§ 35(7)** The syntax `<-` is evidence of a source-level transfer request at this call
site, but the analysis must consume the resolved semantic ownership operation.

**§ 35(8)** For example:

```sec
Consume(CreateItem())
```

**§ 35(9)** also contributes consumption demand even though no `<-` marker is required for
the fresh temporary.

**§ 35(10)** Partial moves should retain canonical Place precision where Sec ownership rules
permit partial use.

---

## § 36 Copy use

**§ 36(1)** A source operation that currently causes a copy does not automatically establish
`CopyRequired`.

**§ 36(2)** The analysis must determine whether semantic independence of the copy is itself
required.

**§ 36(3)** If the copied local is only used for read-only observation that could have been
performed through a borrow, the implementation may still have
`BorrowSufficient` demand.

**§ 36(4)** Likewise, ordinary by-value parameter syntax is not evidence of
`ConsumptionRequired`.

**§ 36(5)** A by-value parameter may create source-level copy semantics when its argument is
an existing reusable Place and the value is copyable.

**§ 36(6)** Parameter usage analysis asks what the implementation semantically requires,
not merely what the current declaration causes at its boundary.

---

## § 37 Whole-value operations

**§ 37(1)** Some operations observe or transfer the complete semantic value.

**§ 37(2)** Examples may include:

- value equality;
- hashing;
- serialization;
- copy/move construction;
- destruction-sensitive transfer;
- compiler-known whole-value operations.

**§ 37(3)** `WholeValue` observation remains distinct from ownership.

**§ 37(4)** A function may require the whole value but still only require borrowed read
access.

---

## § 38 Calls propagate demand

**§ 38(1)** Parameter usage analysis is interprocedural.

**§ 38(2)** For a direct call:

```sec
Inner(value)
```

**§ 38(3)** caller demand includes the applicable callee demand for the corresponding
parameter.

**§ 38(4)** Conceptually:

```text
Demand(value) =
    Join(
        local uses,
        Instantiate(Demand(Inner.parameter0), value)
    )
```

**§ 38(5)** This propagation must preserve relevant Place/projection information where the
callee summary is projection-sensitive.

**§ 38(6)** Propagation uses the resolved callee parameter contract.

**§ 38(7)** For a non-consuming ordinary by-value or borrowed parameter, the caller demand
is instantiated according to that contract.

**§ 38(8)** For a consuming `->` parameter, caller demand includes the required ownership
transfer.

**§ 38(9)** When the actual argument is an existing reusable Place, valid source syntax
contains `<-` at the call site.

**§ 38(10)** When the actual argument is a fresh temporary, the same consuming contract may
be satisfied without `<-`.

**§ 38(11)** The demand summary therefore stores semantic transfer requirements and must not
store "move marker present" as the ownership model.

---

## § 39 Forwarding

**§ 39(1)** A parameter that is only forwarded still inherits callee requirements.

**§ 39(2)** Example:

```sec
fn Outer(values: int[1000]) int {
    return Inner(values)
}
```

**§ 39(3)** If `Inner` requires only a call-local read-only sequence, `Outer` may have the
same minimum demand.

**§ 39(4)** If `Inner` consumes ownership, `Outer` must retain the stronger demand.

**§ 39(5)** If the source parameter of `Outer` is itself available as an owned named Place,
forwarding it into an explicitly consuming `Inner` parameter uses the canonical
call-site transfer marker:

```sec
fn Inner(-> value: Value) void {
    ...
}

fn Outer(-> value: Value) void {
    Inner(<-value)
}
```

**§ 39(6)** This example describes source syntax only.

**§ 39(7)** The propagated `ParameterDemand` remains expressed as semantic ownership demand,
not as syntax tokens.

---

## § 40 Control-flow joins

**§ 40(1)** Demands from all semantically reachable paths are conservatively joined.

**§ 40(2)** Example:

```sec
if store {
    Save(<-value)
} else {
    Inspect(value)
}
```

**§ 40(3)** This example assumes `Save` has a consuming parameter contract:

```sec
fn Save(-> value: Value) void
```

**§ 40(4)** The consuming branch therefore requires the combined parameter demand to retain
consumption capability.

**§ 40(5)** requires consumption capability because one reachable path consumes the value.

**§ 40(6)** The total demand is the minimum combined contract sufficient for every reachable
use, which means joining toward the strongest capability required by any path.

---

## § 41 Unreachable paths

**§ 41(1)** Proven unreachable semantic paths do not contribute demand.

**§ 41(2)** This includes paths eliminated by canonical compile-time reasoning before the
parameter demand result is finalized.

**§ 41(3)** Ordinary optimizer speculation must not change source validity or semantic
demand.

---

## § 42 Loops

**§ 42(1)** A use inside a loop contributes the same semantic capability requirement as a
use outside the loop.

**§ 42(2)** Loop frequency is not part of semantic demand.

**§ 42(3)** Frequency may later affect recommendation ranking.

**§ 42(4)** Loop-carried parameter and forwarding facts must participate in the analysis
fixed point where required.

---

## § 43 Recursive functions

**§ 43(1)** Recursive and mutually recursive parameter-demand summaries are solved to a
sound fixed point.

**§ 43(2)** The compiler should reuse canonical call-graph SCC information or an equivalent
declaration-order-independent mechanism.

**§ 43(3)** Results must not depend on source declaration order.

---

## § 44 Function-value calls

**§ 44(1)** For:

```sec
operation(value)
```

**§ 44(2)** parameter usage analysis consumes callable-flow facts and callable contracts
from closure/callable analysis.

**§ 44(3)** If concrete targets are known, the corresponding parameter demands are joined.

**§ 44(4)** If an open callable contract is available, the analysis consumes the contract.

**§ 44(5)** If a critical demand dimension is absent from the contract, that dimension
becomes conservative `Unknown`.

**§ 44(6)** The analysis must not invent a non-retaining or borrow-only contract for an
unresolved callable target.

---

## § 45 Dimension-specific unknown

**§ 45(1)** Unknown information is tracked per demand dimension.

**§ 45(2)** A result such as:

```text
Access = Read
Shape = ContiguousSequence
Ownership = UnknownOwnership
Lifetime = UnknownLifetime
Identity = AddressRequired
```

**§ 45(3)** is valid.

**§ 45(4)** The compiler must not collapse all independently known facts to a single total
`Unknown` merely because one dimension is unresolved.

**§ 45(5)** This rule is especially important across FFI, callable, and separate-compilation
boundaries.

---

## § 46 FFI is an explicit contract boundary

**§ 46(1)** Parameter usage analysis may propagate semantic demand across an FFI boundary
only from:

- explicit foreign contracts;
- compiler-known foreign semantics;
- resolved ABI/layout requirements where those requirements are normative for
  the foreign operation.

**§ 46(2)** The analysis must not infer ownership, retention, mutability, or lifetime from
C-like pointer syntax alone.

---

## § 47 Foreign call-only access

**§ 47(1)** If a foreign contract explicitly guarantees:

```text
ReadOnly
CallOnly
NoRetention
```

**§ 47(2)** then a foreign call may contribute:

```text
Access = Read
Ownership = BorrowSufficient
Lifetime = CallOnly
```

**§ 47(3)** plus any representation/storage requirements.

---

## § 48 Foreign storage requirements

**§ 48(1)** A foreign operation may independently require:

```text
Contiguous
Aligned
StableAddress
Pinned
SpecificMemorySpace
ExactRepresentation
```

**§ 48(2)** Therefore a parameter can be semantically borrow-sufficient while still
requiring strong storage constraints.

**§ 48(3)** Example conceptual demand:

```text
Access = Read
Ownership = BorrowSufficient
Lifetime = CallOnly
Identity = AddressRequired
Storage = Contiguous + Aligned + Pinned
```

**§ 48(4)** A generic sequence abstraction would be insufficient if it cannot guarantee
those storage properties.

---

## § 49 Foreign retention

**§ 49(1)** If a foreign contract allows retention after the call, the parameter demand must
reflect it.

**§ 49(2)** Example:

```text
MayRetainUntilExplicitRelease
```

**§ 49(3)** may require:

```text
Lifetime = ForeignRetention
Identity = StableIdentityRequired
```

**§ 49(4)** plus any ownership or storage requirements defined by the FFI contract.

---

## § 50 Missing foreign retention information

**§ 50(1)** When retention behavior is not declared and cannot be obtained from a
compiler-known foreign contract:

```text
Lifetime = UnknownLifetime
```

**§ 50(2)** for affected reference/address/view-like dependencies.

**§ 50(3)** The analysis must not treat missing information as `CallOnly`.

**§ 50(4)** This may block a narrowing recommendation even if other dimensions are known.

---

## § 51 Foreign ownership transfer

**§ 51(1)** Foreign ownership transfer must be explicit.

**§ 51(2)** When a foreign API consumes ownership, the parameter demand includes the
corresponding `OwnershipRequired` or `ConsumptionRequired` fact according to the
normative FFI ownership model.

**§ 51(3)** Foreign pointers must not be assumed borrowed merely because they are passed as
addresses.

---

## § 52 FFI pointer-plus-length wrappers

**§ 52(1)** A foreign pointer-plus-length API is an important narrowing case.

**§ 52(2)** Example:

```sec
fn Write(data: byte[4096]) Result[...] {
    return raw_write(data.Ptr, data.len)
}
```

**§ 52(3)** If the foreign contract proves:

```text
read-only
contiguous
call-only
no retention
```

**§ 52(4)** then parameter usage analysis may derive:

```text
Access = Read
Ownership = BorrowSufficient
Lifetime = CallOnly
Shape = ContiguousSequence
ExactExtent = NotRequired
```

**§ 52(5)** This is a strong candidate for a borrowed contiguous sequence/view API.

**§ 52(6)** The analysis must still preserve any required alignment, pointer, memory-space,
or representation constraints.

---

## § 53 FFI callbacks/context

**§ 53(1)** Foreign callback registration may create transitive parameter demand.

**§ 53(2)** Example conceptually:

```text
ForeignRegister(callback, context)
```

**§ 53(3)** may retain both values according to its foreign contract.

**§ 53(4)** Parameter usage analysis consumes:

- closure/callable environment dependencies;
- escape retention summaries;
- FFI callback/context contracts.

**§ 53(5)** It does not reimplement closure or escape analysis.

---

## § 54 Compiler-known operations

**§ 54(1)** Compiler-known/core operations expose demand summaries or contracts through the
same analysis interface used for ordinary callees.

**§ 54(2)** A compiler-known operation may require:

- whole-value read;
- address formation;
- contiguous storage;
- exact layout;
- ownership transfer;
- another explicitly defined capability.

**§ 54(3)** Parameter usage analysis must not maintain ad-hoc hard-coded guesses for such
operations when canonical compiler-known contracts are available.

---

## § 55 Demand joins

**§ 55(1)** Demand joins are conservative.

**§ 55(2)** Conceptually:

```text
Read + Write
    => Write-capable access

BorrowSufficient + ConsumptionRequired
    => ConsumptionRequired

Sequence + RandomAccessSequence
    => RandomAccessSequence

CallOnly + Retained
    => Retained
```

**§ 55(3)** The lattice may retain multiple independent dimensions rather than forcing all
facts into a single total ordering.

**§ 55(4)** The combined result must be sufficient for every reachable operation.

---

## § 56 Monotonicity and termination

**§ 56(1)** Interprocedural demand propagation is monotone.

**§ 56(2)** As additional reachable requirements are discovered, the required capability
may become stronger or more conservative.

**§ 56(3)** A possible required capability must never disappear because another path is
weaker.

**§ 56(4)** Recursive summaries must terminate through finite demand domains, bounded
projection precision, conservative widening, or an equivalent sound method.

---

## § 57 Candidate narrowing

**§ 57(1)** Candidate narrowing is derived only after `RequiredDemand` has been established.

**§ 57(2)** The compiler conceptually tests:

```text
Capabilities(candidate)
    >=
RequiredDemand(parameter)
```

**§ 57(3)** A candidate is valid only if every required capability remains available.

**§ 57(4)** The analysis never rewrites the declaration automatically.

---

## § 58 Canonical narrowing classes

**§ 58(1)** The analysis should be able to express at least the following conceptual
candidate classes where the Sec type system provides a suitable form:

```text
ByValueToSharedReference
ByValueToMutableReference
FixedArrayToSharedView
FixedArrayToMutableView
OwningSequenceToSharedView
OwningSequenceToMutableView
BroaderSequenceContract
ReducedExtentContract
```

**§ 58(2)** The exact source spelling is owned by the relevant reference/array/slice/type
rulebooks.

**§ 58(3)** A candidate may exist as an analysis fact even when Sec 0.1 has no exact source
type representing the theoretical weakest contract.

---

## § 59 Fixed array to view

**§ 59(1)** A fixed-array parameter is a strong narrowing candidate when the implementation
requires sequence access but not exact extent, ownership, or structural
mutation.

**§ 59(2)** Example:

```sec
fn Sum(values: int[4096]) int {
    let mut total := 0

    for i in 0..<values.len {
        total += values[i]
    }

    return total
}
```

**§ 59(3)** may produce:

```text
Access = Read
Mutation = NoMutation
Ownership = BorrowSufficient
Lifetime = CallOnly
Identity = ValueOnly
Shape = RandomAccessSequence
ExactExtent = NotRequired
```

**§ 59(4)** A shared sequence view is then a valid candidate if the source-language view
contract provides all required capabilities.

---

## § 60 Mutable fixed array to mutable view

**§ 60(1)** If the implementation mutates elements but does not require ownership,
structural mutation, or exact extent, a mutable sequence view may be a valid
candidate.

**§ 60(2)** The analysis must not recommend a shared view for a parameter whose reachable
uses require mutation.

---

## § 61 Owning sequence to borrowed view

**§ 61(1)** An owning collection parameter may be stronger than required when the callee
only reads or mutates elements during the call and never requires structural
ownership.

**§ 61(2)** A borrowed candidate is invalid when any path performs structural mutation,
ownership transfer, retention that requires ownership, or another unsupported
operation.

---

## § 62 Large value to reference

**§ 62(1)** `BorrowSufficient` does not imply that a reference is always a useful API.

**§ 62(2)** Example:

```sec
fn Double(value: int) int {
    return value * 2
}
```

**§ 62(3)** A shared reference would normally be worse than a value parameter even though
ownership is unnecessary.

**§ 62(4)** Recommendation policy must therefore distinguish semantic sufficiency from
cost and API quality.

---

## § 63 Semantic demand and recommendation policy are separate

**§ 63(1)** The semantic analysis produces the same `RequiredDemand` independently of
whether the user wants advisories.

**§ 63(2)** Recommendation policy may consume additional facts such as:

```text
T.SizeOf
AlignOf(T)
copy/move cost
construction/destruction cost
call frequency
API visibility
caller conversion cost
CompilationPlan
project analysis configuration
```

**§ 63(3)** These facts influence presentation and ranking.

**§ 63(4)** They do not redefine the semantic demand.

---

## § 64 ResolvedLayout as cost input

**§ 64(1)** `ResolvedLayout` may be used to estimate whether by-value transfer is large or
expensive.

**§ 64(2)** Layout size alone must not change facts such as:

```text
BorrowSufficient
ConsumptionRequired
ExactExtent
Retained
```

**§ 64(3)** unless the operation has an explicit layout/representation requirement.

**§ 64(4)** A change in target layout may therefore invalidate recommendation/cost data
without invalidating semantic demand.

---

## § 65 Recommendation reasons

**§ 65(1)** A narrowing opportunity may carry one or more reasons such as:

```text
AvoidCopyCost
ReduceOwnershipRequirement
IncreaseAcceptedInputs
RemoveUnusedExactExtent
RemoveUnusedMutability
RemoveUnusedStructuralCapability
ReduceRetentionRequirement
SimplifyFFIWrapperContract
```

**§ 65(2)** Reasons are analysis/tooling facts rather than new source-language semantics.

---

## § 66 Explicit contracts and programmer intent

**§ 66(1)** A parameter type may intentionally express a stronger public invariant than the
current body happens to exercise.

**§ 66(2)** Examples include:

- cryptographic fixed blocks;
- protocol-defined fixed headers;
- constrained named types;
- hardware buffers;
- exact FFI representation;
- explicit ownership-signaling APIs;
- target-specific storage contracts.

**§ 66(3)** Parameter usage analysis distinguishes:

```text
UnusedByCurrentImplementation
```

**§ 66(4)** from:

```text
SemanticallyRedundant
```

**§ 66(5)** The latter requires a stronger proof.

**§ 66(6)** An explicit normative contract suppresses incompatible narrowing candidates.

---

## § 67 Recommendation confidence

**§ 67(1)** Recommendation output may classify confidence conceptually as:

```text
Certain
Strong
Heuristic
```

**§ 67(2)** `Certain` means the stronger capability is proven unnecessary under the
applicable contracts.

**§ 67(3)** `Strong` means a narrower API is strongly supported by implementation demand,
but API intent remains a design choice.

**§ 67(4)** `Heuristic` means cost or design evidence suggests an opportunity without a
fully compelling API conclusion.

**§ 67(5)** Exact scoring/ranking is compiler policy.

---

## § 68 Unknown critical dimensions block narrowing

**§ 68(1)** A narrowing candidate must not be recommended when a required capability is
unknown and the candidate relies on that capability being absent.

**§ 68(2)** Example:

```text
candidate = shared borrow
Lifetime = UnknownLifetime
```

**§ 68(3)** The compiler must not recommend the candidate as proven sufficient.

**§ 68(4)** Deep analysis may instead report a blocked candidate:

```text
A shared borrow may be sufficient, but retention behavior of `X` is unknown.
```

---

## § 69 Candidate blockers

**§ 69(1)** Analysis should be able to explain why a plausible narrower contract was
rejected.

**§ 69(2)** Examples:

```text
shared view blocked because ExactExtent(64) is required
shared reference blocked because one path consumes ownership
borrowed buffer blocked because foreign code may retain the address
mutable view blocked because the callee may grow the collection
```

**§ 69(3)** Blocker reporting is especially useful in Deep analysis.

---

## § 70 Ownership syntax in recommendation fixes

**§ 70(1)** A source edit proposed by parameter usage analysis must use canonical current
ownership syntax.

**§ 70(2)** In particular:

- changing a parameter to consuming form uses `->`;
- transferring an existing reusable Place at a call site uses `<-`;
- a code action must not insert `<-` merely because a value is move-only;
- a code action must not remove `<-` when the resolved callee parameter is
  consuming and the source is an existing reusable Place;
- a fresh temporary passed to a consuming parameter must not receive synthetic
  move boilerplate;
- return rewrites must respect that `return value` already transfers an owned
  result where ordinary return semantics permit it.

**§ 70(3)** A code action changing a function parameter contract must update affected call
sites only when the compiler has proven that the complete edit set remains
semantically valid.

---

## § 71 Caller conversion cost

**§ 71(1)** Recommendation policy may consider caller-side conversion cost.

**§ 71(2)** A candidate is not automatically preferable if every caller must perform an
expensive allocation, copy, representation change, or another semantically
significant conversion.

**§ 71(3)** For array-to-view narrowing, conversion is often cheap and therefore likely to
be useful.

**§ 71(4)** Other candidates may require stronger cost analysis.

---

## § 72 API visibility

**§ 72(1)** Recommendation policy may rank public/exported API opportunities more strongly
than private helper opportunities.

**§ 72(2)** The semantic `RequiredDemand` remains identical.

**§ 72(3)** Visibility only influences presentation and ranking.

---

## § 73 Whole-program call frequency

**§ 73(1)** When available, whole-program call frequency may strengthen an efficiency
advisory.

**§ 73(2)** For example, a large by-value parameter used at many call sites may deserve
higher priority than the same parameter in an infrequently used helper.

**§ 73(3)** Call count is optional analysis information.

**§ 73(4)** It is not required for semantic demand or baseline correctness.

---

## § 74 Narrowing groups

**§ 74(1)** Deep analysis may group interprocedural narrowing opportunities.

**§ 74(2)** Example:

```text
A(data: LargeArray)
    -> B(data: LargeArray)
        -> C(data: LargeArray)
```

**§ 74(3)** If all three functions require only call-local read-only sequence access, Deep
analysis may report one `NarrowingOpportunityGroup` rather than three unrelated
advisories.

**§ 74(4)** This is an optional precision/usability capability.

---

## § 75 Analysis budgets

**§ 75(1)** Parameter usage analysis follows the compiler-wide analysis-budget model:

```text
Interactive
Standard
Deep
```

**§ 75(2)** These budgets affect cost, precision, and presentation.

**§ 75(3)** They do not define different source-language semantics.

---

## § 76 Interactive analysis

**§ 76(1)** Interactive analysis is primarily intended for LSP use.

**§ 76(2)** It may use:

- cached summaries;
- incremental invalidation;
- bounded local/interprocedural propagation;
- conservative demand summaries;
- limited recommendation computation.

**§ 76(3)** Interactive analysis must remain sound for any facts presented as proven.

**§ 76(4)** Incomplete recomputation may be represented as tooling state such as `Pending`.

**§ 76(5)** `Pending` must never be treated as a semantic proof.

---

## § 77 Standard analysis

**§ 77(1)** Standard compilation performs all parameter-demand computation required by
other mandatory compiler analyses and by the active `CompilationPlan`.

**§ 77(2)** Parameter design advisories may be suppressed or limited according to project
policy.

**§ 77(3)** Standard compilation must not flood normal build output with low-value API
suggestions.

---

## § 78 Deep analysis

**§ 78(1)** Deep analysis is the natural mode for exhaustive parameter-usage review.

**§ 78(2)** It may perform:

- larger interprocedural propagation budgets;
- whole-program call-frequency analysis;
- caller conversion-cost analysis;
- cross-module narrowing groups;
- public API scoring;
- more precise range/shape demand;
- context-sensitive callable refinement;
- blocked-candidate reporting;
- extended cause paths and reasoning.

**§ 78(3)** Deep analysis refines the same semantic demand model.

**§ 78(4)** It does not define a different Sec language.

---

## § 79 `sec analyse`

**§ 79(1)** Parameter usage analysis participates in the compiler-wide analysis command.

**§ 79(2)** Conceptually:

```text
sec analyse
```

**§ 79(3)** runs all available analyses by default.

**§ 79(4)** The explicit equivalent is conceptually:

```text
sec analyse --all
```

**§ 79(5)** The user may also request parameter usage analysis specifically.

**§ 79(6)** Individual selection is spelled `sec analyse parameter-usage`
(`rules/tooling/cli.md` § 5.2, MD-017).

**§ 79(7)** Deep parameter-usage output may include:

- declared capability;
- required demand;
- narrowing candidates;
- blocked candidates;
- relevant FFI boundaries;
- reasoning/cause paths;
- interprocedural narrowing groups;
- optional size/cost/call-frequency information.

---

## § 80 LSP presentation

**§ 80(1)** LSP presentation distinguishes:

```text
NeedToKnow
OptionalInsight
```

**§ 80(2)** Parameter usage information is normally `OptionalInsight` unless needed to
explain another diagnostic or required semantic condition.

**§ 80(3)** Default hover should not dump the complete demand lattice merely because the
compiler knows it.

**§ 80(4)** A concise hover may show facts such as:

```text
Read-only, call-only parameter
```

**§ 80(5)** or:

```text
Fixed extent is not required by this function.
```

**§ 80(6)** Detailed/Deep LSP mode may expose full demand/candidate information.

**§ 80(7)** Call counts, estimated traffic, and similar enrichment are optional and
configurable.

---

## § 81 Project-configured LSP depth

**§ 81(1)** Project configuration may control parameter-analysis depth and presentation.

**§ 81(2)** A small project on a high-capacity machine may enable Deep parameter usage
analysis continuously.

**§ 81(3)** A large project or resource-constrained machine may use cheaper Interactive
analysis.

**§ 81(4)** The LSP integration must consume the project configuration through the common
LSP configuration mechanism.

**§ 81(5)** The detailed requirements for reading, watching, reloading, invalidating, and
recomputing analysis state belong to the LSP rulebook and its synchronization
appendix.

**§ 81(6)** Project-file syntax and configuration precedence are owned by
`rules/projects/projects.md`.

**§ 81(7)** Parameter usage analysis consumes the resolved project/CompilationPlan analysis
policy and does not define a competing manifest syntax.

---

## § 82 Function summaries

**§ 82(1)** Each analyzable function exposes a stable parameter-usage summary.

**§ 82(2)** Conceptually:

```text
ParameterUsageSummary {
    Parameters
    Receiver
    AnalysisPrecision
}
```

**§ 82(3)** Each parameter may expose conceptually:

```text
ParameterSummary {
    DeclaredCapability
    RequiredDemand
    CandidateNarrowings
    BlockedNarrowings
}
```

**§ 82(4)** The exact representation is implementation-defined.

**§ 82(5)** `RequiredDemand` is the persistent semantic core.

**§ 82(6)** Candidate narrowings and recommendation messages are derived information.

---

## § 83 Receiver demand

**§ 83(1)** Methods analyze `self`/receiver demand explicitly.

**§ 83(2)** Receiver use must not be hidden merely because it is not written as an ordinary
source parameter.

**§ 83(3)** The same access, mutation, ownership, lifetime, identity, shape, storage, and
representation dimensions apply where meaningful.

---

## § 84 Separate compilation

**§ 84(1)** Parameter-demand propagation must continue across compilation-unit boundaries.

**§ 84(2)** When callee bodies are unavailable, the compiler consumes persisted summaries or
explicit normative contracts.

**§ 84(3)** Imported summary information should preserve dimension-specific facts such as:

```text
Read
BorrowSufficient
CallOnly
ContiguousSequence
UnknownLifetime
```

**§ 84(4)** rather than collapsing the entire parameter to `Unknown`.

---

## § 85 Inferred and declared demand contracts

**§ 85(1)** Demand may originate from:

```text
InferredDemand
DeclaredDemandContract
```

**§ 85(2)** Inferred demand comes from an analyzable Sec body.

**§ 85(3)** Declared demand contracts are required where body analysis is unavailable or
where another normative boundary owns the behavior, such as FFI or opaque
compiler-known code.

**§ 85(4)** Where both exist and the language permits verification, the compiler should
validate that the declared guarantee is compatible with the implementation.

---

## § 86 Persisted summary versioning

**§ 86(1)** Persisted parameter usage summaries require an internal schema/version identity.

**§ 86(2)** A summary must be compatible with at least:

```text
function identity
function signature
relevant generic specialization
relevant CompilationPlan identity
summary schema/version
```

**§ 86(3)** An incompatible stale summary must not be used as current analysis evidence.

**§ 86(4)** If the body is available, the summary may be recomputed.

**§ 86(5)** Otherwise the compiler falls back to an explicit contract or conservative
unknown behavior.

---

## § 87 Summary invalidation

**§ 87(1)** A cached semantic-demand summary is invalidated when relevant inputs change,
including:

- function body;
- callee demand summary;
- callable target/contract;
- FFI contract;
- ownership semantics;
- escape/lifetime summary;
- storage requirement;
- type semantics used by the demand;
- generic specialization;
- CompilationPlan-selected implementation;
- analysis schema/version.

**§ 87(2)** Recommendation/cost caches may additionally depend on:

- `ResolvedLayout` size/alignment;
- call frequency;
- API visibility;
- project presentation policy;
- caller conversion-cost facts.

**§ 87(3)** Semantic-demand and recommendation caches may therefore be invalidated
independently.

---

## § 88 CompilationPlan

**§ 88(1)** Semantic parameter demand is target-independent when the reachable semantics are
target-independent.

**§ 88(2)** A `CompilationPlan` becomes part of the analysis identity when it changes the
relevant implementation or normative requirements, for example through:

- target-selected code;
- target-specific FFI contracts;
- hardware storage requirements;
- target-known compiler operations;
- target-specific layout/representation contracts.

**§ 88(3)** Cost/recommendation data may be target-specific even when semantic demand is
unchanged.

---

## § 89 Generic functions

**§ 89(1)** Generic parameter-demand summaries may be shared when their requirements are
independent of concrete type semantics.

**§ 89(2)** A specialization-specific summary is required when demand depends on, for
example:

- copy/move properties;
- concrete aggregate structure;
- storage requirements;
- callable contracts;
- FFI layout;
- target-selected implementation.

**§ 89(3)** The compiler may reuse summaries when semantic equivalence is proven.

---

## § 90 Diagnostics and advisories

**§ 90(1)** Parameter usage diagnostics are primarily advisory/informational.

**§ 90(2)** A stronger source parameter contract remains valid unless another rulebook says
otherwise.

**§ 90(3)** Canonical conceptual diagnostic categories include:

```text
parameter.unused-capability
parameter.unnecessary-by-value
parameter.unnecessary-ownership
parameter.unnecessary-mutable-access
parameter.unnecessary-fixed-extent
parameter.owning-sequence-where-view-suffices
parameter.fixed-array-where-view-suffices
parameter.narrowing-blocked
parameter.ffi-wrapper-overconstrained
```

**§ 90(4)** User-facing diagnostic identity, severity, configurability, structured
occurrences, help, fixes, CLI transport, and LSP transport are governed by
`rules/tooling/diagnostics.md`.

**§ 90(5)** The category strings above are stable internal advisory categories of parameter
usage analysis. They are not automatically compiler diagnostic IDs.

**§ 90(6)** Every emitted primary diagnostic must use a registered stable diagnostic ID.

**§ 90(7)** Published IDs must not be repurposed merely because a parameter-usage category
would fit them approximately.

---

## § 91 Diagnostic explanations

**§ 91(1)** A useful recommendation explains the proof.

**§ 91(2)** Example:

```text
information: parameter `values` does not require its fixed-array contract

The function:
    reads elements
    performs indexed access
    does not mutate the collection
    does not retain or return a dependency on it
    does not require exactly 4096 elements

A shared sequence view is sufficient for the analyzed implementation.
```

**§ 91(3)** The compiler should prefer evidence-based explanations over terse commands such
as:

```text
use slice instead
```

---

## § 92 Large-value advisory

**§ 92(1)** For a large by-value value whose implementation only requires a borrow, an
advisory may explain both semantic and cost evidence.

**§ 92(2)** Example conceptually:

```text
information: parameter `customer` does not require ownership

The function only reads `customer` during the call.
No reachable operation copies for semantic independence, moves, retains,
mutates, or depends on stable identity.

`Customer` is expensive to pass by value on the active CompilationPlan.
Consider a shared reference parameter if borrowed access matches the intended
API.
```

**§ 92(3)** The cost threshold is compiler policy, not a language constant.

---

## § 93 FFI wrapper advisory

**§ 93(1)** FFI-related diagnostics should expose the actual foreign requirement.

**§ 93(2)** Example:

```text
information: wrapper parameter is stronger than the foreign operation requires

Foreign contract requires:
    read-only contiguous bytes
    stable address during the call
    no retention

`buffer` additionally requires callers to provide an owned fixed array of
exactly 4096 bytes.
```

**§ 93(3)** This makes foreign requirements visible without requiring the programmer to
manually reconstruct them from ABI details.

---

## § 94 Blocked narrowing advisory

**§ 94(1)** Deep analysis may explain a blocked candidate.

**§ 94(2)** Example:

```text
candidate: shared sequence view

not proven because:
    `ForeignRegister` may retain the supplied address
```

**§ 94(3)** or:

```text
candidate: shared reference

blocked because:
    `StoreCustomer` consumes ownership on one reachable path
```

**§ 94(4)** Blocked candidates are optional analysis output and are not normal compiler
errors.

---

## § 95 Consumers

**§ 95(1)** Parameter usage analysis produces facts for:

- LSP;
- `sec analyse`;
- API-review tooling;
- documentation tooling;
- recommendation ranking;
- compiler cost reporting;
- optimization planning where source semantics remain unchanged.

**§ 95(2)** Backend lowering may consume semantic demand as additional information.

**§ 95(3)** It must not use a narrowing recommendation to silently change the source-level
function contract.

---

## § 96 Inputs from other analyses

**§ 96(1)** Parameter usage analysis consumes canonical facts from:

- Place/provenance analysis;
- ownership/copy/move analysis;
- lifetime analysis;
- escape analysis;
- closure/callable-flow analysis;
- call graph;
- storage analysis;
- layout resolution;
- effect analysis;
- FFI contracts;
- compiler-known operation contracts.

**§ 96(2)** It must not create competing local models for those domains.

---

## § 97 Required Standard implementation capabilities

**§ 97(1)** For Sec 0.1, Standard parameter usage analysis must support at least:

- ordinary parameters;
- method receivers;
- field reads and writes;
- fixed-array and slice/view indexing;
- iteration;
- collection element mutation;
- structural collection mutation;
- moves and ownership consumption;
- semantic copy requirements;
- returns;
- retention and escaping dependencies;
- direct call propagation;
- function-value calls through known targets or callable contracts;
- branches;
- loops;
- recursive and mutually recursive call SCCs;
- fixed arrays;
- slices/views;
- ordinary aggregates;
- address requirements;
- storage requirements;
- foreign contract boundaries;
- dimension-specific unknown demand;
- summary production and consumption.

---

## § 98 Optional Deep capabilities

**§ 98(1)** Deep parameter usage analysis may additionally provide:

- whole-program call frequency;
- cross-module narrowing groups;
- caller conversion-cost analysis;
- hot-path weighting;
- public API scoring;
- context-sensitive callable refinement;
- more precise known-range/shape demand;
- extended blocker/cause-path output.

**§ 98(2)** These improve precision and usability.

**§ 98(3)** They are not required to define the semantics of `ParameterDemand`.

---

## § 99 Required positive/no-advisory tests

**§ 99(1)** The test suite must include cases where narrowing must not be recommended,
including at least:

```text
small cheap value parameter
parameter ownership consumed
parameter retained
fixed extent semantically required
structural collection mutation
stable identity required
pinned storage required
specific memory-space requirement
foreign ownership transfer
foreign retention
observable independent copy requirement
explicit hardware/representation contract
```

**§ 99(2)** These tests protect against over-aggressive recommendations.

---

## § 100 Required narrowing tests

**§ 100(1)** The test suite must include at least:

```text
large read-only struct -> shared-reference candidate
mutable large struct -> mutable-reference candidate
fixed array read-only -> shared-view candidate
fixed array element mutation -> mutable-view candidate
owning sequence read-only -> shared-view candidate
owning sequence element mutation -> mutable-view candidate
unused exact extent
unused ownership
unused mutability
```

---

## § 101 Required shape tests

**§ 101(1)** The test suite must include at least:

```text
iteration only -> Sequence
indexing -> RandomAccessSequence
fixed index -> MinimumExtent, not ExactExtent
length observation -> not automatically ExactExtent
explicit fixed-size contract -> preserve ExactExtent
known partial range -> retain range demand
contiguous foreign call -> ContiguousSequence
```

---

## § 102 Required interprocedural tests

**§ 102(1)** The test suite must include at least:

```text
direct forwarding
transitive forwarding
callee consumes
callee retains
callee only reads
mixed branch demands
loop-carried demand
recursive SCC
mutual recursion
declaration-order independence
function-value call with known targets
function-value call with open contract
unknown critical dimension blocks narrowing
separate-compilation demand summary
stale summary rejection
```

---

## § 103 Required FFI tests

**§ 103(1)** The FFI parameter-demand matrix must include at least:

```text
read-only call-only pointer
mutable call-only pointer
pointer + length
contiguous requirement
alignment requirement
stable-address requirement
pinned requirement
specific memory space
foreign retention
foreign ownership transfer
unknown retention
unknown ownership
callback/context retention
exact foreign layout requirement
```

**§ 103(2)** It must also include wrapper opportunities where a foreign contract proves that:

```text
fixed array -> borrowed view
owning buffer -> borrowed contiguous view
```

**§ 103(3)** is a valid candidate.

---

## § 104 Required recommendation-policy tests

**§ 104(1)** The same semantic `ParameterDemand` must remain stable when:

```text
advisories are disabled
minimum confidence changes
copy-cost target changes
call-frequency information is unavailable
public/private recommendation policy changes
LSP uses Interactive mode
sec analyse uses Deep mode
```

**§ 104(2)** These tests ensure presentation settings do not leak into semantic analysis.

---

## § 105 Required ownership-transfer tests

**§ 105(1)** Revision 2.0 requires at least:

```text
consuming parameter declared with ->
existing reusable Place passed with <-
fresh temporary passed to consuming parameter without <-
ordinary by-value parameter not inferred as consuming from move-only type alone
return value recognized as transfer without mandatory <-
optional return <-value produces equivalent ownership demand when legal
consuming forwarding propagates ConsumptionRequired
branch join with one consuming path retains ConsumptionRequired
parameter-demand summaries encode semantic consumption rather than syntax-token presence
code action never inserts synthetic <- before a fresh temporary
code action does not remove required <- from an existing reusable Place
diagnostic/advisory categories remain distinct from stable compiler diagnostic IDs
```

---

## § 106 False-positive regression tests

**§ 106(1)** The compiler must maintain regression cases where strong parameter types are
intentional, including representative cases such as:

```text
cryptographic fixed block
network protocol fixed header
hardware register/buffer contract
FFI ABI struct
constrained domain aggregate
ownership-signaling API
```

**§ 106(2)** The analysis must preserve explicit semantic intent instead of blindly
minimizing every parameter type.

---

## § 107 Analysis-tier tests

**§ 107(1)** Interactive tests must verify:

```text
sound bounded result
incremental invalidation compatibility
Pending tooling state does not become a semantic proof
```

**§ 107(2)** Standard tests must verify:

```text
deterministic semantic demand
required interprocedural propagation
required FFI contract handling
```

**§ 107(3)** Deep tests must verify:

```text
same source semantics
same or stronger precision
additional advisory information only
no weaker safety assumptions
```

---

## § 108 Determinism

**§ 108(1)** For the same:

- source program;
- active `CompilationPlan`;
- compiler version;
- analysis configuration relevant to Standard semantic precision;

**§ 108(2)** Standard parameter-demand results must be deterministic.

**§ 108(3)** Worklist ordering, thread scheduling, cache hit order, or similar incidental
implementation details must not change the semantic result.

---

## § 109 Completion criteria for Sec 0.1

**§ 109(1)** Parameter usage analysis is complete for Sec 0.1 when all of the following hold:

1. multidimensional parameter demand can be represented;
2. ordinary local operations produce the required demand facts;
3. copy, move, borrow, ownership, and consumption are distinguished;
4. element mutation and structural collection mutation are distinguished;
5. sequence use, random access, contiguity, exact extent, and minimum extent are
   distinguished where required;
6. escape/lifetime retention facts are consumed rather than duplicated;
7. address, storage, and representation requirements are preserved;
8. direct-call demand propagates interprocedurally;
9. function-value calls use known target summaries or callable contracts;
10. FFI behaves as an explicit contract boundary;
11. missing information remains dimension-specific and conservative;
12. branch, loop, recursion, and mutual-recursion fixed points are sound;
13. separate compilation can consume versioned parameter-demand summaries;
14. candidate narrowing preserves every required capability;
15. explicit source contracts and API invariants suppress incompatible advice;
16. semantic demand remains separate from cost/recommendation policy;
17. Interactive, Standard, and Deep consumers use the same semantic model;
18. required positive, narrowing, shape, interprocedural, FFI, policy, and
    false-positive regression tests pass.

---

## § 110 Governance

**§ 110(1)** This rulebook contains normative parameter-usage analysis behavior and
completion requirements.

**§ 110(2)** Mutable implementation progress belongs to:

```text
governance/analysis.yaml
```

**§ 110(3)** under:

```text
sema.parameter-usage-analysis
```

**§ 110(4)** That integration should track granular capabilities including at least:

```text
local demand
control-flow demand
direct-call propagation
consuming-parameter transfer demand
return-boundary transfer demand
recursive demand
array/view narrowing
large-value narrowing
FFI contract demand
callable-contract propagation
summary persistence
Deep whole-program analysis
diagnostic integration
LSP integration
```

**§ 110(5)** Implementation status owned by another subsystem must remain in that subsystem's
governance entry.

**§ 110(6)** In particular:

```text
copy/move frontend semantics
    declarations/memory ownership governance owner

diagnostic registry and transport
    governance/errors_diagnostics.yaml

FFI contracts
    FFI governance owner

LSP presentation
    governance/tooling_lsp.yaml

incremental summary invalidation
    incremental-compilation governance owner
```

**§ 110(7)** `sema.parameter-usage-analysis` may record dependencies on those integrations
but must not duplicate their mutable implementation state.

---

## § 111 Final normative summary

**§ 111(1)** Parameter usage analysis determines the minimum semantic capabilities required
from each parameter and receiver.

**§ 111(2)** The analysis is multidimensional and independently tracks access, mutation,
ownership, lifetime/retention, identity, shape, storage, representation, and
precision.

**§ 111(3)** The analysis distinguishes the parameter contract currently declared by the
programmer from the weaker or equal contract required by the implementation.

**§ 111(4)** It never silently changes the function signature.

**§ 111(5)** Source-level parameter semantics remain distinct from physical ABI argument
passing.

**§ 111(6)** Demand propagates through direct calls, callable contracts, control flow, loops,
recursion, FFI boundaries, compiler-known operations, and separate-compilation
summaries.

**§ 111(7)** FFI behavior is derived only from explicit or compiler-known foreign contracts.
Missing retention, ownership, or other critical foreign information remains
conservative unknown for the affected dimensions.

**§ 111(8)** Candidate narrowing is permitted only when the candidate preserves every
required capability.

**§ 111(9)** A narrowing opportunity is advisory. It does not prove that the weaker contract
better expresses the programmer's intended public API.

**§ 111(10)** Semantic demand and recommendation policy remain separate. Layout, call
frequency, API visibility, caller conversion cost, project settings, and the
active `CompilationPlan` may affect recommendation ranking without redefining
semantic demand.

**§ 111(11)** The LSP presents parameter-analysis information according to configurable
Interactive/Deep policy and distinguishes information the programmer needs from
optional insight.

**§ 111(12)** `sec analyse` runs parameter usage analysis as part of its default all-analysis
behavior and may also run it individually.

**§ 111(13)** Mutable implementation progress is governed by
`sema.parameter-usage-analysis` in `governance/analysis.yaml`.
