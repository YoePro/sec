# Sec ABI

- **Status:** Normative
- **Created:** 2026-08-16
- **Last updated:** 2026-08-16
- **Document revision:** 1
- **Sec language version:** 0.1
- **Canonical path:** `rules/platform/abi.md`

## 1. Purpose

This rulebook defines the canonical Sec 0.1 Application Binary Interface model.

It specifies how already-valid Sec and foreign call semantics are mapped to
physical calling contracts under a concrete `CompilationPlan`.

It owns:

- resolved ABI identity;
- native Sec, C, and system calling-convention families;
- parameter and return classification;
- direct and indirect transport;
- hidden ABI parameters;
- aggregate call classification;
- stack argument placement requirements;
- foreign ABI physical classification;
- canonical ABI signatures;
- ABI fingerprints and compatibility;
- the contract consumed by Sec MLIR ABI lowering;
- separate-compilation ABI metadata and stale-artifact detection.

This rulebook does not define:

- Sec source type compatibility;
- source ownership, borrowing, move, or destruction semantics;
- whether a type is legal in a particular FFI position;
- source-level foreign resource ownership;
- general storage layout;
- linker symbol spelling, mangling, export policy, or dead stripping;
- MLIR operation syntax or LLVM IR encoding.

Those concerns remain owned by the corresponding type, memory, FFI, layout,
linking, Sec MLIR, and backend rulebooks.

---

## 2. Core principle

ABI rules answer:

> How is an already semantically valid value or callable physically transported
> across this concrete call boundary?

ABI classification does not decide whether a program is semantically valid.

In particular, ABI classification does not define:

- whether two Sec types are compatible;
- whether a value is copied, moved, or borrowed;
- whether an FFI parameter or return type is legal;
- whether a foreign pointer may be retained;
- whether a source-level conversion exists.

Those decisions occur before ABI lowering.

---

## 3. CompilationPlan authority

Every concrete `CompilationPlan` resolves one authoritative `ABIModel`.

Downstream compiler stages must consume that resolved model.

They must not independently infer ABI rules from:

- operating-system names;
- architecture names;
- compiler-host properties;
- backend defaults;
- target triples alone;
- foreign function names.

The platform resolver may use operating system, architecture, CPU, target
profile, device, and explicit configuration when selecting an ABI, but the
resolved `ABIModel` is thereafter authoritative.

---

## 4. ABI families

Sec 0.1 recognizes the following source-facing calling-convention families:

```text
Sec
C
system
```

They correspond to:

```sec
extern "Sec" fn NativeEntry(value: int) int
extern "C" fn ForeignEntry(value: C::int) C::int
extern "system" fn PlatformEntry(value: C::int) C::int
```

The active `CompilationPlan` resolves each family to a concrete calling
convention model.

### 4.1 Sec ABI

The Sec ABI is the physical interface used by native Sec calls when no other
calling convention is explicitly required.

It may use native Sec representations and need not be C-compatible.

### 4.2 C ABI

The C ABI is the active target C binary interface.

It owns the physical representation and call classification of compiler-known
C types and C-compatible foreign declarations.

### 4.3 System ABI

The system ABI is the active platform system calling convention.

It may be physically identical to the active C ABI on a particular
`CompilationPlan`, but Sec does not define `system` as a synonym for `C`.

---

## 5. Sec ABI stability

Sec 0.1 does not promise native Sec ABI compatibility across arbitrary compiler
or ABI-schema versions.

A supported compiler generation must nevertheless produce deterministic ABI
contracts for the same:

- source semantics;
- `CompilationPlan`;
- ABI schema;
- compiler semantic version relevant to ABI lowering.

Persisted native Sec artifacts must carry enough ABI identity to reject
incompatible use after an ABI-breaking compiler change.

---

## 6. Layout and ABI classification

Storage layout and call ABI classification are separate concerns.

Canonical layout answers questions such as:

```text
Size
Align
Stride
FieldOffsets
```

ABI classification answers questions such as:

```text
Direct or indirect transport?
How many ABI parts?
Which machine classes?
Which extension rules?
Which stack requirements?
Is hidden return storage required?
```

ABI classification consumes canonical resolved representation and layout facts.

It does not maintain a competing layout engine.

Two semantic types with the same `Size` and `Align` are not necessarily
classified identically if the selected ABI distinguishes their representation
categories.

---

## 7. Semantic type and transport representation

ABI transport does not replace semantic type identity.

Conceptually, the compiler keeps distinct facts equivalent to:

```text
SemanticType
ResolvedPhysicalRepresentation
ABITransportRepresentation
```

A named Sec type may therefore have the same physical ABI transport as its
underlying representation while remaining a distinct Sec type.

For example:

```sec
type CustomerId uint64
type OrderId uint64
```

may classify identically in a particular ABI without becoming semantically
interchangeable.

The same rule applies to constrained named types and unit-bearing types when
their canonical representation introduces no additional per-value storage.

---

## 8. ABI classification model

ABI classification is position-sensitive.

A conforming implementation provides the semantic equivalent of:

```text
ClassifyParameter(type, callingConvention)
ClassifyReturn(type, callingConvention)
ClassifyVarArg(type, callingConvention)
```

The same semantic type may classify differently as:

- an ordinary parameter;
- a return value;
- a variadic foreign argument.

A generic `GetABIType(type)` operation is insufficient when the ABI distinguishes
call positions.

---

## 9. Canonical argument classification

ABI argument classification is represented independently of any specific
Sec MLIR, standard MLIR, LLVM dialect, or LLVM IR encoding.

A conceptual result may contain information equivalent to:

```text
ABIArgInfo {
    PassingKind
    Parts
    Alignment
    Extension
    TransportAttributes
}
```

The exact implementation representation is not normative.

### 9.1 Passing kinds

At minimum, the ABI model must be able to distinguish semantics equivalent to:

```text
Direct
Indirect
Ignore
```

`Direct` does not mean "one register".

`Indirect` does not mean that the Sec source parameter became a pointer.

`Ignore` is valid only where the selected ABI defines no transported value for
that call position.

---

## 10. Direct transport

A directly transported value may consist of one or more canonical ABI parts.

Conceptually:

```text
Direct [
    ABIValuePart(IntegerClass, offset 0, width 64),
    ABIValuePart(IntegerClass, offset 8, width 64)
]
```

An ABI part may describe:

- byte or bit offset within the semantic value representation;
- machine class;
- transport width;
- alignment requirement;
- coercion requirement where defined by the ABI.

The ABI layer should use target-neutral machine classes rather than directly
encoding LLVM types.

Concrete physical register names are not required in the canonical ABI
classification unless another target-specific rule explicitly requires them.

---

## 11. Indirect transport

An ABI may transport an aggregate or result indirectly through compiler-managed
storage.

Conceptually, an indirect classification may record facts equivalent to:

```text
Indirect {
    StorageRole
    RequiredAlignment
    CopyIn
    CopyOut
}
```

Possible storage roles include caller-provided argument temporaries and
caller-provided return storage.

This storage is ABI machinery.

It does not create:

- source-level `RawPtr[T]`;
- a new Sec owner;
- a Sec borrow;
- a source-visible lifetime;
- a new semantic copy or move.

---

## 12. Hidden return storage

If the selected ABI returns a value through hidden return storage, ABI lowering
may materialize an implicit result-storage parameter.

The source function remains:

```sec
fn Build() LargeValue
```

rather than becoming a source signature equivalent to:

```sec
fn Build(result: RawPtr[LargeValue]) void
```

Return classification occurs early enough during whole-call planning to account
for any hidden parameters or resources introduced by the return convention.

---

## 13. Machine transport is not Sec copy/move semantics

ABI lowering may:

- copy bits into argument registers;
- split an aggregate;
- spill or reload values;
- materialize outgoing argument storage;
- copy to or from ABI temporaries.

These operations do not create additional semantic Sec copies, moves, owners, or
borrows.

The source-level copy/move/borrow decision has already been made by the
canonical Sec semantic model.

ABI lowering preserves it.

---

## 14. Scalar classification

Scalar ABI classification may determine:

- machine class;
- transport width;
- register eligibility;
- stack representation;
- sign extension;
- zero extension;
- other ABI-mandated transport attributes.

An ABI transport extension is not a Sec numeric conversion.

It does not change the semantic source type or source value domain.

---

## 15. Aggregate classification

Aggregate classification consumes:

- semantic representation category where ABI-relevant;
- canonical resolved layout;
- field or element categories where required by the ABI;
- field offsets;
- aggregate size;
- aggregate alignment;
- packing state;
- homogeneous-aggregate rules where applicable;
- the selected ABI's classification algorithm.

Sec defines no universal rule equivalent to:

```text
aggregate size <= N => pass in registers
```

Each `ABIModel` owns its canonical aggregate algorithm.

Aggregate classification may produce multiple ABI parts without changing the
aggregate's semantic type.

---

## 16. References and pointer-like values

ABI classification uses the resolved physical representation of the actual
semantic type.

It must not assume:

```text
ref T == one native machine pointer
```

A native Sec safe reference may carry generation, provenance, or other
compiler-defined representation information under a concrete
`CompilationPlan`.

`RawPtr[T]` follows the canonical raw-pointer representation of the applicable
address space.

Transporting a `RawPtr[T]` does not imply:

- ownership;
- lifetime;
- bounds;
- non-nullness.

---

## 17. Native Sec descriptors and aggregates

The Sec ABI may transport native Sec runtime representations such as:

- `string`;
- owning dynamic arrays;
- compiler-known collection descriptors;
- native structs;
- native enums;
- native tagged unions;
- `Result`;
- `Option`;
- native callable values.

ABI classification consumes their resolved representations.

It does not independently define:

- descriptor field layout;
- enum representation;
- tagged-union layout;
- `Result` layout;
- `Option` niche optimization;
- callable environment layout.

Those are owned by the appropriate semantic and layout rules.

---

## 18. Native values are not automatically foreign-legal

Physical representability under an ABI does not imply FFI legality.

For example, the Sec ABI may validly transport:

```sec
fn Process(value: string) void
```

while an `extern "C"` function may reject `string` unless an explicit canonical
FFI rule defines a legal foreign representation.

ABI lowering must not automatically invent adaptations such as:

```text
string -> char*
list[T] -> pointer + length
```

Such adaptation belongs to explicit FFI or wrapper semantics.

---

## 19. C scalar representation

Compiler-known C scalar types retain distinct semantic identities such as:

```text
C::int
C::long
C::long_double
C::bool
```

Their physical representations are selected by the active C `ABIModel`.

Sec scalar types such as:

```text
bool
char
rune
int
uint
```

do not become their C counterparts merely because their physical layouts happen
to match on a particular target.

---

## 20. C aggregates

Foreign C structs, unions, enums, bitfields, flexible arrays, and other
supported C data forms use the active C ABI's canonical data-layout and call
classification rules.

The C ABI model should expose coherent interfaces for:

```text
C data representation
C aggregate layout
C parameter classification
C return classification
C variadic classification
```

These interfaces may be separate implementation components, but they share one
resolved C ABI identity and must not drift.

FFI remains responsible for deciding whether a particular C-facing source form
is legal.

---

## 21. Foreign `ref` adaptation

FFI may permit a foreign declaration such as:

```sec
extern "C" fn Inspect(value: ref Header) C::int
```

when `ref Header` denotes the canonical non-null, call-bounded foreign borrow
contract.

At that verified boundary, ABI lowering may use the foreign address
representation required by the selected C or system ABI rather than the complete
native Sec safe-reference representation.

This is an explicit FFI/ABI boundary adaptation.

It does not redefine native `ref T` representation.

It does not permit arbitrary reference erasure at ordinary Sec call boundaries.

---

## 22. Native callable and foreign function pointer

Native Sec callable types and foreign C function pointers are distinct semantic
and physical representation families.

Conceptually:

```text
fn(C::int) void
```

and:

```text
C::fn(C::int) void
```

are not ABI-identical merely because both are callable.

A native callable uses the resolved Sec callable representation.

A C function pointer uses the active C ABI function-pointer representation.

---

## 23. Callback bridges

A compiler-generated foreign callback thunk is an explicit ABI bridge.

Conceptually it has:

```text
Foreign-facing ABI contract
        ↓
validated physical adaptation
        ↓
Sec-facing ABI contract
```

The bridge may:

- unpack foreign ABI arguments;
- construct the already-approved Sec-side representation;
- invoke the Sec callable;
- adapt the already-approved Sec result back to the foreign ABI.

It must not invent semantic conversions or ownership changes that were not
validated by FFI and Sema.

A bridge therefore has two compatible semantic boundaries and two concrete ABI
contracts rather than one ambiguous mixed ABI.

---

## 24. Variadic foreign arguments

Foreign variadic legality and C default promotions are established by the
canonical FFI/C rules before physical ABI classification.

The ABI model then classifies the promoted argument according to the selected
ABI's:

- variadic register rules;
- stack rules;
- register-save requirements;
- position-specific classification differences.

C varargs remain distinct from native Sec typed variadics.

---

## 25. Whole-call ABI planning

Type classification and concrete call assignment are separate stages.

The canonical ABI classification describes how each value may or must be
transported.

Whole-call assignment uses the complete call shape and ABI resource state to
determine final argument placement.

Conceptually, assignment may maintain:

```text
IntegerArgumentResources
FloatingArgumentResources
VectorArgumentResources
OutgoingStackOffset
```

The canonical model may use abstract resource indices rather than concrete
machine register names.

Register exhaustion and stack fallback follow the selected ABI's rules.

A split aggregate may not be partially assigned to registers and stack unless
the selected ABI explicitly allows that form.

---

## 26. Stack arguments

Stack-passed values receive explicit ABI-defined physical placement information,
including the equivalent of:

```text
StackOffset
Size
Alignment
```

The ABI model owns outgoing call-stack argument layout.

Later lowering must not recompute it using a generic approximation such as:

```text
offset += sizeof(argument)
```

without applying the selected ABI's actual rules.

---

## 27. Padding

Padding required by physical layout or ABI transport is not semantic Sec data.

ABI lowering preserves the physical contract without:

- creating source-visible fields;
- treating padding as initialized semantic values;
- changing type identity.

Any explicit security, initialization, or foreign-boundary rule that constrains
padding remains owned by its canonical rulebook.

---

## 28. CallABIPlan and Sec MLIR staging

The `ABIModel` is available through the `CompilationPlan` before Sec MLIR ABI
lowering.

High-level Sec MLIR retains:

- semantic function shape;
- semantic parameter and result types;
- calling-convention intent;
- already-resolved layout and representation facts required by later lowering.

It does not have to materialize the final callable ABI representation
immediately.

During the canonical Sec MLIR ABI-lowering stage, the compiler produces the
equivalent of a `CallABIPlan`.

Conceptually:

```text
Resolved semantic call
        ↓
ABIModel classification
        ↓
whole-call ABI assignment
        ↓
CallABIPlan
        ↓
ABI-lowered MLIR
        ↓
lower MLIR / LLVM dialect
        ↓
LLVM IR
```

The exact in-memory representation of `CallABIPlan` is implementation-specific.

---

## 29. CallABIPlan contents

A call plan contains enough information to materialize the complete physical
call contract.

Conceptually it includes:

```text
CallABIPlan {
    CallingConvention
    Return
    Parameters
    HiddenParameters
    VariadicState
    StackRequirements
}
```

The plan may additionally preserve semantic-to-physical correspondence for
diagnostics, verification, and debug information.

---

## 30. Call-plan construction algorithm

A conforming implementation should follow staging equivalent to:

```text
BuildCallABIPlan(signature, callingConvention, compilationPlan):

    abi = compilationPlan.ResolveABI(callingConvention)

    classify the return representation
    materialize required hidden return ABI parameters

    classify each explicit parameter

    if foreign variadic arguments exist:
        consume the already-promoted legal variadic forms
        classify them using the ABI's variadic rules

    create whole-call assignment state

    assign hidden parameters according to ABI order
    assign explicit parameters according to ABI order
    assign variadic arguments according to ABI order

    finalize outgoing stack layout
    finalize required call attributes

    verify the complete call plan

    return the verified plan
```

The exact function decomposition is non-normative.

---

## 31. Caller/callee consistency

A caller and callee derived from the same:

- semantic callable;
- calling convention;
- `CompilationPlan`;
- ABI model version;

must produce compatible ABI contracts.

Within one compilation generation, a mismatch between caller and callee
representations derived from the same declaration is a compiler invariant
failure.

Foreign declarations may additionally fail because imported ABI metadata or
foreign declarations are inconsistent.

---

## 32. Canonical ABI signature

Every concrete callable lowered across an ABI boundary has a canonical ABI
signature describing its physical call contract.

Conceptually:

```text
ABISignature {
    CallingConvention
    ReturnClassification
    ParameterClassifications
    HiddenParameters
    VariadicKind
    RequiredStackProperties
}
```

An ABI signature is distinct from the source semantic signature.

Two semantically different declarations may have physically equivalent ABI
signatures without becoming semantically interchangeable.

---

## 33. ABI compatibility

ABI compatibility is a physical binary-interface relation.

It does not replace:

- Sec type compatibility;
- callable compatibility;
- interface conformance;
- FFI semantic compatibility.

The compiler may provide an operation equivalent to:

```text
AreABISignaturesCompatible(A, B)
```

but the result is meaningful only for physical call compatibility.

---

## 34. ABI fingerprints

A concrete canonical ABI signature may be assigned an `ABIFingerprint` suitable
for:

- incremental cache identity;
- separate compilation;
- stale-artifact detection;
- binary compatibility verification.

The fingerprint includes all ABI-relevant facts whose changes may alter the
physical contract, including as applicable:

- ABI identity;
- ABI schema identity;
- calling convention;
- parameter classifications;
- return classification;
- hidden ABI parameters;
- variadic classification;
- relevant physical representation facts.

It excludes source locations and presentation-only names.

Fingerprint equality may implement or accelerate compatibility checking, but
ABI compatibility is defined by the canonical physical contract rather than by
a particular hash implementation.

---

## 35. Semantic identity, ABI identity, and symbol identity

The following are distinct:

```text
SemanticDeclarationIdentity
ABISignature / ABIFingerprint
BinarySymbolIdentity
```

`modules.md` owns semantic module and declaration identity.

This rulebook owns physical ABI contract identity.

`rules/compiler/linking.md` owns final binary symbol and artifact linkage semantics.

A semantic declaration may retain the same identity while its ABI fingerprint
changes.

Likewise, public semantic visibility does not itself require an externally
visible binary symbol.

---

## 36. Separate compilation

Separate-compilation artifacts that expose callable binary boundaries provide
enough canonical metadata to verify the expected ABI without reparsing source
text.

Conceptually this includes:

```text
SemanticDeclarationIdentity
SemanticSignatureIdentity
ABISignature
ABIFingerprint
ABI schema / model identity
CompilationPlan-relevant ABI identity
```

The serialization format is not defined here.

Semantic `ModuleSurface` and binary ABI surface are separate concepts.

`ModuleSurface` exists for Sec semantic import resolution.

ABI metadata exists for physical compatibility across compiled boundaries.

---

## 37. Stale artifact detection

A compiler or linker pipeline must not silently combine incompatible native Sec
artifacts merely because their linker symbol spelling still matches.

When persisted ABI metadata proves that caller and callee contracts differ, the
artifact is stale or incompatible and must be rebuilt or rejected.

Sec ABI schema incompatibility is a sufficient reason to reject a persisted
native artifact when compatibility is not explicitly defined.

---

## 38. Incremental invalidation

An implementation-only function change that preserves the callable ABI contract
does not require downstream invalidation solely for ABI reasons.

A change that modifies an ABI signature or ABI fingerprint invalidates every
cached or persisted result that depends on the prior physical contract.

Possible invalidation dependencies include:

- concrete layout changes;
- ABI model changes;
- calling-convention changes;
- target or `CompilationPlan` changes;
- reference representation changes;
- foreign ABI representation changes;
- parameter or result signature changes;
- hidden ABI parameter changes;
- ABI schema changes.

---

## 39. MLIR ABI verification

After the Sec MLIR ABI-lowering stage materializes the physical call
representation, a verifier checks equivalence to the canonical `CallABIPlan`
and `ABISignature`.

The verifier should validate the semantic equivalent of:

```text
calling convention identity
return transport
hidden parameters
parameter part count and order
parameter transport classes
stack placements
variadic ABI state
required call attributes
```

Later standard MLIR, LLVM-dialect, LLVM IR, and backend stages preserve this
verified contract.

They do not independently rederive a competing Sec ABI.

---

## 40. Debug-information correspondence

ABI lowering preserves enough correspondence between:

```text
semantic source parameter/result
```

and:

```text
physical ABI components
```

for diagnostics and debug information.

A source function such as:

```sec
fn Process(value: CustomerId) bool
```

continues to appear as that source-level function to the debugger even when the
physical call contract introduces:

- hidden storage;
- split parts;
- widened argument transport;
- machine-only temporaries.

The exact debug-information encoding belongs to the debug-information rulebook.

---

## 41. Determinism

For the same:

- semantic callable;
- resolved layout and representation facts;
- calling convention;
- `CompilationPlan`;
- ABI schema/model identity;

the compiler produces the same:

- argument classifications;
- return classification;
- hidden parameters;
- whole-call assignment;
- stack layout;
- `CallABIPlan`;
- `ABISignature`;
- `ABIFingerprint`.

The result is independent of:

- map iteration order;
- source traversal order;
- compiler worker scheduling;
- compiler host architecture.

---

## 42. Implementation boundaries

A conforming compiler should preserve the following layering:

```text
Sec semantic model
        ↓
canonical layout / representation
        ↓
ABIModel
        ↓
canonical ABI classification
        ↓
Sec MLIR ABI lowering
        ↓
CallABIPlan + ABI-lowered MLIR
        ↓
lower MLIR / LLVM dialect
        ↓
LLVM IR / backend
```

The ABI rules are not implemented independently in every layer.

The canonical `ABIModel` provides the physical contract.

Sec MLIR materializes it.

Later lowering preserves it.

---

## 43. Required test families

A conforming Sec 0.1 ABI implementation includes regression coverage for at
least the following areas.

### 43.1 Scalar transport

Test:

- signed and unsigned fixed-width integers;
- pointer-sized integer representations where plan-dependent;
- floating scalars;
- sign and zero extension;
- native Sec `bool`;
- C scalar families under multiple ABIs;
- raw pointers.

### 43.2 Aggregate transport

Test:

- zero-sized/ignored forms where canonical rules permit them;
- small direct aggregates;
- multi-part aggregates;
- large indirect aggregates;
- alignment-sensitive aggregates;
- nested aggregates;
- aggregates with ABI-relevant mixed scalar classes;
- return versus parameter classification differences.

### 43.3 References and descriptors

Test:

- native safe references using their complete resolved representation;
- raw pointers;
- foreign call-bounded `ref` adaptation;
- native strings and collection descriptors;
- rejection of automatic foreign string/slice decomposition where FFI does not
  define it.

### 43.4 Calling-convention families

Test:

- native Sec ABI;
- C ABI;
- system ABI;
- plans where system and C resolve to the same physical model;
- plans where they resolve differently.

### 43.5 Hidden ABI machinery

Test:

- hidden return storage;
- indirect parameters;
- ordering of hidden and explicit parameters;
- stack alignment;
- stack fallback;
- no accidental semantic ownership changes.

### 43.6 C and foreign behavior

Test:

- `C::` scalar representation;
- foreign structs;
- foreign unions;
- foreign enums;
- foreign function pointers;
- callbacks and generated bridges;
- foreign varargs after canonical promotions.

### 43.7 Cross-module and persisted ABI

Test:

- caller/callee signature equality;
- incompatible ABI metadata;
- stale object detection;
- Sec ABI schema mismatch;
- compatible implementation-only edits;
- ABI-changing layout edits;
- multi-Variant plans with different ABI signatures.

### 43.8 MLIR boundary

Test:

- high-level Sec MLIR retains semantic callable shape before ABI lowering;
- ABI lowering creates the correct `CallABIPlan`;
- emitted ABI-lowered MLIR matches the plan;
- verifier rejects mismatched parameter parts, result transport, hidden
  parameters, calling conventions, and variadic state;
- lower MLIR/LLVM stages preserve rather than reclassify the ABI contract.

### 43.9 Determinism

Test identical ABI results under different:

- source traversal orders;
- map insertion orders;
- parallel compilation schedules;
- compiler hosts during cross compilation.

---

## 44. Completion criteria

The Sec 0.1 ABI implementation is complete when all supported concrete
`CompilationPlan`s can:

1. resolve one canonical `ABIModel`;
2. classify native Sec, C, and system call positions;
3. classify scalars, references, aggregates, descriptors, and supported foreign
   representations from canonical resolved representations;
4. preserve source-level ownership, borrow, copy, move, and type identity while
   producing physical ABI transport;
5. build complete `CallABIPlan`s during Sec MLIR ABI lowering;
6. verify ABI-lowered calls and function entries against those plans;
7. derive canonical `ABISignature` and `ABIFingerprint` values;
8. verify caller/callee compatibility across separately compiled boundaries;
9. reject stale or ABI-incompatible persisted artifacts;
10. invalidate ABI-dependent compiler artifacts when the physical contract
    changes;
11. preserve the verified ABI through lower MLIR, LLVM dialect, LLVM IR, and
    backend generation;
12. produce deterministic results independent of compiler-host or scheduling
    accidents.

---

## 45. Non-goals for Sec 0.1

This rulebook does not require:

- stable native Sec ABI compatibility across arbitrary compiler releases;
- automatic C compatibility for ordinary Sec types;
- implicit native descriptor decomposition at foreign boundaries;
- source-visible ABI temporary pointers;
- source ownership reconstruction from machine calling mechanics;
- hard-coded physical register names in the target-neutral ABI model;
- a second ABI implementation inside LLVM lowering;
- linker export policy or symbol mangling rules;
- a universal aggregate-size threshold shared by all targets.
