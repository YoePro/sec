# Sec Monomorphization

- Status: Canonical normative rulebook
- Created: 2026-09-04
- Last updated: 2026-09-04
- Document revision: 1.0
- Language version: Sec 0.1
- Canonical path: `rules/compiler/monomorphization.md`

## 1. Purpose

This rulebook defines canonical monomorphization semantics for Sec compile-time type generics.

It defines how one valid generic template and one canonical complete generic substitution produce or reuse one canonical concrete semantic instantiation; how specialization demand is discovered and tracked; how concrete generic identities interact with analysis, linking, ABI, FFI, incremental compilation, diagnostics, and tooling; and how distinct semantic instantiations may share physical implementation without losing their semantic identity.

Sec generics are compile-time specialization. They do not require runtime generic parameters, universal generic type descriptors, generic dictionaries, implicit boxing, type erasure, or runtime generic dispatch.

This rulebook governs Sec 0.1 type-generic specialization. It does not introduce const/value generics or user-defined compile-time execution.

This rulebook does not redefine generic declaration syntax, constraint syntax, generic inference, ordinary type semantics, ownership, destruction, layout, ABI classification, FFI legality, linker mechanics, effect domains, or stack algorithms. Those remain owned by their canonical rulebooks.

## 2. Cross-rulebook ownership

### 2.1 Source generics

`rules/declarations/generics.md` owns:

- generic declaration syntax;
- generic parameter and argument syntax;
- supported generic declaration families;
- generic parameter scope;
- constraints and constraint satisfaction;
- explicit and inferred generic arguments;
- method-level generic parameter syntax;
- generic template validity;
- unsupported source-level generic mechanisms.

Determining that a source use resolves to a substitution such as:

```text
T -> Packet
```

belongs to generic resolution and inference.

### 2.2 Monomorphization

This rulebook owns:

- canonical instantiation identity;
- canonical instantiation keys;
- demand-driven concrete specialization;
- the instantiation dependency graph;
- application of an already-resolved canonical substitution;
- specialization reuse;
- nested specialization demand;
- specialization termination;
- semantic specialization versus physical realization;
- physical implementation-sharing policy;
- generic-specific cross-module specialization behavior;
- generic cache and fingerprint invariants;
- generic diagnostic identity and provenance;
- the generic side of ABI and FFI boundaries.

### 2.3 Generic lowering

`rules/compiler/generics_lowering.md` owns the verified compiler boundary from an already canonical concrete specialization into representation-dependent lowering.

It consumes and preserves canonical specialization identity. It does not redefine that identity or generic demand semantics.

## 3. Core model

Sec distinguishes the following compiler entities:

```text
GenericTemplate
        +
CanonicalSubstitution
        ↓
ConcreteInstantiation
        +
CompilationPlan
        ↓
PlanRealization
        ↓
zero or more PlanEntries
        ↓
zero or more ImplementationBodies
```

These levels are related but not interchangeable.

A generic template is a compile-time semantic declaration. A concrete instantiation is one canonical semantic specialization. A plan realization adds target-, ABI-, layout-, optimization-, and backend-specific facts. A plan entry is a physical callable boundary when one is required. An implementation body is executable implementation and may be shared by multiple semantic instantiations.

A concrete semantic instantiation does not imply one emitted function, one binary symbol, one machine address, or one physical implementation body.

## 4. Canonical instantiation identity

### 4.1 Instantiation identity

Every complete concrete specialization has one canonical semantic identity.

Conceptually, an instantiation identity is determined by:

```text
CanonicalGenericDeclarationIdentity
+
OrderedCanonicalConcreteGenericArguments
+
CanonicalSpecializationRole where required by the declaration kind
```

The exact compiler data structure is implementation-defined.

For example:

```text
Pair[string, int]
Pair[int, string]
```

are different instantiations because argument order is significant.

### 4.2 Nominal identity

Nominal generic identity is preserved independently of physical representation.

If:

```sec
type UserId int32
type ProductId int32
```

then:

```text
Foo[UserId]
Foo[ProductId]
```

are distinct semantic instantiations even when both arguments have the same physical representation.

### 4.3 Phantom parameters

A generic parameter contributes to canonical generic identity even when it does not contribute runtime storage.

Two concrete generic enum, interface, named-type, struct, union, or other nominal instances with different canonical arguments remain distinct semantic types even when those arguments are phantom with respect to representation.

### 4.4 What is not identity

The following do not redefine canonical instantiation identity:

- source spelling;
- whether an argument was explicit or inferred;
- requesting module;
- request or discovery order;
- compiler worker identity;
- cache entry identity;
- source line or byte offset;
- target layout;
- ABI fingerprint;
- binary symbol spelling;
- machine address;
- implementation-sharing class;
- optimization profile.

### 4.5 Explicit and inferred arguments

If explicit and inferred generic arguments produce the same canonical substitution, they identify the same semantic instantiation.

For example, where inference yields `T = int32`:

```sec
let a := Identity[int32](10)
let b := Identity(10)
```

both refer to the same canonical `Identity[int32]` specialization.

The compiler may preserve explicit-versus-inferred information as source provenance.

## 5. Template validity

### 5.1 Template analysis precedes specialization

A generic declaration is semantically validated as a template under its declared generic contracts before concrete monomorphization.

Monomorphization specializes a valid generic declaration. It does not make an otherwise invalid generic declaration legal merely because one particular concrete argument happens to provide an undeclared operation.

### 5.2 No deferred duck typing

This is invalid unless the generic contract explicitly permits `+` for `T`:

```sec
fn Invalid[T](value: T) T {
    return value + value
}
```

The compiler must not accept the template on the basis that some future instantiations might happen to support the operation.

### 5.3 Constraints define the template environment

For:

```sec
fn Serialize[T: Serializable](value: T) Result[void, IOError] {
    return value.Serialize()
}
```

template analysis may rely on the capabilities guaranteed by `Serializable`.

Concrete specialization later resolves the canonical conformance and the concrete operation selected for the concrete argument.

### 5.4 Explicit guarantees

An explicit semantic guarantee declared by a generic template applies to every valid substitution.

Concrete specialization may refine an inferred implementation fact but must not weaken an explicit declared guarantee.

## 6. Demand-driven specialization

### 6.1 Canonical demand

Concrete specialization is demand-driven.

A canonical concrete instantiation becomes active only when required by the current semantic program or by an explicit tooling query that requests semantic specialization.

Template analysis by itself does not demand all possible concrete instantiations.

### 6.2 Repeated demand

Repeated requests for the same canonical instantiation reuse one semantic instantiation.

If multiple modules request:

```text
Foo[Packet]
```

within the same applicable declaration universe, the requesting module does not create a second semantic specialization.

### 6.3 Tooling and caches do not create program demand

A cached or speculative specialization created for hover, completion, analysis inspection, or a previous build does not become part of the active program merely because it exists.

Cache presence or tooling analysis must not create:

- execution reachability;
- LinkRoots;
- static-storage materialization;
- binary emission;
- current-program instantiation demand.

### 6.4 Generic methods

Generic methods use the same demand model as generic functions and generic types.

For:

```sec
impl Stack[T] {
    fn Map[U](mapper: fn(T) U) Stack[U] {
        // ...
    }
}
```

`Stack[int32].Map[string]` has an enclosing substitution and a method-level substitution. Both participate in canonical specialization identity according to their semantic roles.

Unused concrete generic methods do not require physical code emission merely because the enclosing generic type is used.

## 7. Instantiation dependency graph

### 7.1 Graph nodes

The instantiation dependency graph contains canonical concrete instantiations demanded by the semantic program.

Edges record one specialization requiring another concrete specialization.

For example:

```text
Outer[Packet]
    -> Inner[Packet]
```

records concrete specialization demand, not necessarily a runtime call.

### 7.2 Cause and provenance

Each demand edge must preserve enough cause information to support diagnostics, incremental invalidation, and compiler inspection.

A dependency may arise from, among other things:

- a direct generic call;
- a nested generic type;
- a field or variant type;
- a generic associated member;
- a generic method;
- destruction or copy support;
- closure or callable construction;
- another compiler-generated semantic dependency.

### 7.3 Instantiation graph is not the call graph

The instantiation dependency graph and execution call graph are separate structures.

A specialization may require another specialization for type or compiler-semantic reasons without creating a runtime call edge.

Conversely, an already-existing concrete specialization may recursively call itself without creating additional generic instantiations.

### 7.4 Instantiation graph is not the LinkPlan

The instantiation graph records semantic specialization dependencies.

The final `LinkPlan` records surviving physical artifacts after legal inlining, dead-code elimination, sharing, internalization, and other lowering or optimization decisions.

## 8. Canonical substitution

### 8.1 Simultaneous semantic substitution

Specialization applies one canonical simultaneous substitution to semantic entities.

Substitution is not textual search-and-replace and must not depend on replacement order.

Generic parameters are canonical semantic symbols.

### 8.2 Scope of substitution

The substitution applies to every generic-dependent semantic entity required by the specialization, including as applicable:

- parameter types;
- result types;
- local types;
- field and variant types;
- nested generic arguments;
- properties;
- associated members;
- impl targets;
- generic method environments;
- constraint-dependent references;
- construction and conversion targets;
- compiler-generated semantic dependencies.

### 8.3 Source AST is not required to be cloned

The language semantics do not require an implementation to deep-copy source AST, replace generic tokens, and rerun the entire frontend for each specialization.

A compiler may specialize from a typed generic template or equivalent canonical semantic representation.

## 9. Concrete semantic specialization

### 9.1 Concrete callable signatures

A runtime-relevant concrete callable specialization has a fully concrete semantic signature.

For:

```sec
fn Select[T](first: T, second: T) T {
    // ...
}
```

`Select[int32]` has the concrete semantic signature:

```text
fn(int32, int32) int32
```

Specialization preserves the callable's canonical parameter ownership modes and typed variadic shape. Monomorphization does not normalize a borrowed, forced-consuming, or typed variadic parameter into a different source-level callable contract.

### 9.2 Concrete generic types

A concrete generic type has fully substituted generic-dependent semantic members.

For:

```sec
type Pair[A, B] struct {
    first: A,
    second: B,
}
```

`Pair[int32, string]` has concrete stored field types `int32` and `string`.

### 9.3 Generic impls

A generic impl follows the substitution of its generic owner.

For:

```sec
type Stack[T] struct {
    // ...
}

impl Stack[T] {
    fn Pop() Option[T] {
        // ...
    }
}
```

`Stack[int32].Pop` has result type `Option[int32]`.

### 9.4 Generic enums

Generic enums are canonical Sec 0.1 generic declaration families.

Every complete canonical argument list produces a distinct concrete nominal enum identity. Ordinary enum semantics apply after substitution.

Equally named members of `State[Connection]` and `State[Socket]` belong to different concrete enum types even if both specializations use the same physical representation.

### 9.5 Generic interfaces

A concrete generic interface such as:

```text
Repository[User]
```

is a concrete interface type.

Generic specialization must not be confused with runtime interface dispatch. Any runtime interface representation or dispatch remains governed by the interface rules after generic arguments have been resolved.

### 9.6 Generic named types

A concrete generic named type preserves its canonical nominal identity after substitution.

Physical representation equivalence with another concrete type does not merge the named types or their generic instantiation identities.

## 10. Resolution inside a specialization

### 10.1 Non-dependent references

Semantic references that do not depend on generic parameters should be reused from template analysis rather than arbitrarily resolved again for every specialization.

### 10.2 Generic-dependent references

A generic-dependent reference is resolved using the canonical concrete substitution and the contracts or conformances that made the template operation valid.

Concrete specialization may resolve a constraint-mediated call to a concrete implementation where the owning interface and dispatch rules permit static resolution.

### 10.3 No new arbitrary overload search

Monomorphization must not introduce new source semantics by searching the program for unrelated operations that merely happen to accept a concrete substituted type.

Concrete specialization resolves choices already permitted by the template semantic environment.

### 10.4 Construction and conversion

A generic-dependent construction or conversion must already be legitimate according to generic and conversion rules at template level.

Monomorphization must not defer the question of whether an arbitrary concrete conversion happens to work until backend specialization.

## 11. Ownership, borrowing, destruction, and statics

### 11.1 Owning rulebooks remain authoritative

Monomorphization does not redefine ownership, borrowing, copy/move, lifetime, availability, destruction, or static-storage semantics.

After substitution, the compiler obtains exact concrete facts from the owning canonical rulebooks.

### 11.2 Concrete ownership behavior

A concrete specialization resolves type-dependent semantic operations such as:

- copy;
- move;
- borrow;
- mutable borrow;
- destruction;
- availability consequences;
- compiler-generated ownership helpers.

Physical loads and stores do not redefine these semantic operations.

### 11.3 Template capability requirements

A generic body must not rely on copyability or another ownership capability that its generic contract does not guarantee.

A later concrete copyable substitution does not rescue an invalid template.

### 11.4 Destruction

A template may carry a symbolic generic-dependent destruction requirement.

Concrete specialization resolves the exact destruction semantics for the substituted type. A later optimizer may eliminate a concrete no-op destruction path only after its semantics have been resolved.

### 11.5 Per-instantiation static storage

Per-specialization static storage remains semantically distinct.

Conceptually:

```text
Cache[int32].Count
!=
Cache[string].Count
```

This remains true even when executable code for the two specializations is physically shared.

Initialization, lifetime, destruction, and concurrency behavior of per-instantiation storage must remain attached to the correct concrete storage identity.

## 12. Analysis integration

### 12.1 Analysis levels

Compiler analysis distinguishes:

```text
Template facts
    ↓
Concrete-instantiation facts
    ↓
CompilationPlan-specific facts
```

The exact analysis ownership remains with the relevant analysis rulebooks.

### 12.2 Template summaries

A generic template may carry reusable symbolic or generic-independent analysis facts.

Examples include:

- source control-flow structure;
- non-dependent call targets;
- parameter-to-return provenance;
- symbolic storage or destruction requirements;
- declared guarantees;
- constraint-level effect requirements.

Such summaries are compile-time compiler metadata. They do not introduce runtime generic values or descriptors.

### 12.3 Concrete analysis identity

Every canonical concrete instantiation has its own semantic analysis identity.

Physical implementation sharing must not merge distinct specializations' semantic:

- call-graph identity;
- effect summary identity;
- ownership facts;
- destruction facts;
- escape or lifetime facts;
- ISR context;
- race or deadlock context;
- stack/resource identity;
- diagnostic identity.

Equivalent immutable summary data may be shared internally without merging the semantic entities to which the facts apply.

### 12.4 Call graph

A generic template is not an executable call-graph node.

Reachable concrete specializations are executable semantic nodes for the active `CompilationPlan`.

A call-graph node may therefore combine one target-independent `InstantiationIdentity` with plan-specific execution-analysis identity without making the underlying generic instantiation target-dependent.

### 12.5 Effects

Generic templates are checked against declared effect guarantees using their generic contracts.

A concrete specialization may refine an inferred effect summary when concrete facts are stronger than the template-level upper bound. It must not weaken an explicit declared guarantee.

### 12.6 Stack and other resource analysis

A template may carry symbolic resource requirements. Concrete semantic specialization may resolve type-dependent resource facts. Machine stack, ABI save areas, spills, and other physical resource facts remain plan-specific.

Physical code sharing must not collapse per-specialization semantic resource identity.

## 13. Generic closure and lowering boundary

### 13.1 Generic closure

Monomorphization closes generic semantics before representation-dependent lowering begins.

A runtime-relevant concrete specialization contains no unresolved generic parameters in semantic positions that affect runtime behavior.

### 13.2 Generic closure is not representation closure

A type is concrete because its generic arguments and semantic identity are fully resolved, not because its physical layout is already known.

For example:

```text
Pair[int, RawPtr[Packet]]
```

may be a fully concrete semantic type before the active `CompilationPlan` has fixed all physical size, alignment, field-offset, or ABI facts.

### 13.3 Required concrete facts

Before representation-dependent lowering, runtime-relevant concrete Semantic IR must not retain unresolved generic parameters in:

- callable signatures;
- stored fields or variant payloads;
- nested generic arguments;
- runtime value types;
- generic impl environments;
- generic-dependent overload choices;
- generic-dependent conformance choices;
- generic-dependent construction or conversion choices;
- ownership/copy/move classification;
- destruction responsibility;
- per-instantiation static-storage identity.

### 13.4 Facts that may remain plan-dependent

The generic closure boundary does not itself require all of the following to be physically resolved:

- size;
- alignment;
- field offsets;
- target-sized integer width;
- pointer width;
- ABI classification;
- register classification;
- calling convention;
- machine stack frame;
- machine symbol;
- instruction selection;
- machine code.

Those facts are resolved by their owning plan and lowering stages when required.

### 13.5 Retained templates

The compiler may retain generic template representations containing generic symbols for later specialization, incremental compilation, separate compilation, and tooling.

The prohibition on unresolved generic parameters applies to runtime-relevant concrete specialization, not to retained template metadata.

### 13.6 Closure verification

The compiler should verify generic closure before representation-dependent lowering.

If valid source semantics reach the concrete lowering boundary with an unresolved generic semantic choice because of a compiler bug, that is a compiler invariant failure rather than a request for additional source syntax.

## 14. Physical implementation sharing

### 14.1 Semantic specialization precedes sharing

Distinct canonical semantic instantiations are fully specialized before physical implementation sharing is considered.

Representation equivalence alone is insufficient to merge semantic specialization.

### 14.2 Default optimization direction

Physical implementation reuse is the default optimization direction.

The compiler should avoid unnecessary physical duplication where sharing preserves all required semantics and is profitable under the active `CompilationPlan`.

Legal sharing is not mandatory. The compiler may deliberately create distinct physical implementations when correctness, performance, size, stack, ABI, debugging, profiling, or another plan policy provides a reason.

### 14.3 Implementation equivalence

Implementation sharing may depend on concrete executable facts including:

- selected operations and call targets;
- ownership and destruction lowering;
- layout and ABI;
- concrete constants;
- static-storage references;
- target operations;
- control flow;
- applicable explicit contracts.

Distinct semantic identities remain distinct regardless of sharing.

### 14.4 Generalized shared implementations

A shared implementation may receive minimal realization-only parameters carrying facts already resolved by compilation, such as:

- concrete size;
- concrete alignment;
- selected destructor address;
- selected operation address;
- per-instantiation static-storage address;
- another specific already-resolved realization fact.

Such parameters are backend implementation details.

They must not defer generic type meaning, constraint resolution, overload resolution, ownership classification, destruction selection, or other generic semantics to runtime.

### 14.5 No universal runtime generic environment

Monomorphization must not turn ordinary Sec generics into a universal runtime environment containing generic type descriptors or broad operation dictionaries from which runtime code discovers what `T` means.

Passing one already-selected concrete operation to a shared implementation is not runtime generic resolution.

### 14.6 Entries and bodies

A plan-specific callable entry and an implementation body are distinct entities.

Multiple semantic specializations may have distinct entries or ABI adapters while converging on one shared implementation body.

A shared body need not use a source-visible or externally stable calling convention.

### 14.7 Static state under sharing

Physical code sharing must not merge per-instantiation static storage.

A shared implementation may receive the concrete storage address as a realization parameter if that preserves all initialization, lifetime, destruction, reachability, and concurrency semantics.

### 14.8 Later folding and cloning

Sharing may arise through deliberate compiler generalization, optimization, LTO, or linker-level identical implementation folding.

Conversely, a previously shared body may be cloned or physically specialized later for performance or tooling without changing semantic instantiation identity.

## 15. Symbols, reachability, and separate compilation

### 15.1 Concrete instantiation is not a binary symbol

A canonical semantic specialization does not by itself require a binary symbol or emitted machine-code body.

A specialization may be analysis-only, unreachable, fully inlined, or physically shared.

### 15.2 Binary materialization

A binary entry is materialized only when the active `PlanRealization` and `LinkPlan` require an addressable, callable, retained, exported, FFI-visible, callback-visible, or otherwise link-visible entity.

### 15.3 Binary identity

When a concrete specialization requires binary identity, that identity is derived from canonical semantic identity together with the relevant binary role and plan-specific ABI facts defined by the linking and ABI rulebooks.

Final mangled symbol spelling is a backend representation of binary identity rather than generic semantic identity itself.

### 15.4 Monomorphization does not create LinkRoots

A specialization becomes link-live only through ordinary canonical reachability rules.

Examples include:

- program entry;
- an ordinary live call;
- a required function value;
- an export;
- a callback;
- an interrupt or startup root;
- another explicit `LinkPlan` root.

### 15.5 Shared-body reachability

A shared implementation body is live while at least one live realization requires it.

A dead specialization must not retain instantiation-specific static storage, initialization, dependencies, debug identity, or other artifacts merely because another specialization shares the same executable body.

### 15.6 Separate compilation

A separately compiled public generic API must preserve enough canonical compiler-consumable template information for later concrete specialization by a Sec consumer.

This information may be serialized typed semantic/compiler IR or another equivalent canonical template artifact. Distribution of the original source AST is not required.

A generic template artifact should preserve, as applicable:

- canonical declaration identity;
- generic parameter and constraint information;
- typed template semantics;
- declared contracts;
- semantic dependencies;
- provenance;
- compatibility fingerprints and schema identities.

A generic template artifact is a compilation interface, not a runtime generic function.

### 15.7 Duplicate physical emission

Separate compilation may temporarily produce compatible duplicate physical definitions for one canonical specialization.

Canonical linker or LTO mechanisms may coalesce such definitions. Backend coalescing does not introduce source-level weak-linkage semantics.

## 16. ABI and FFI boundaries

### 16.1 Generic templates have no concrete runtime ABI

An unresolved generic template has no single runtime ABI.

Only a fully concrete specialization may acquire a `CompilationPlan`-specific ABI signature.

### 16.2 ABI identity is separate

`InstantiationIdentity`, semantic callable signature, `ABISignature`, `ABIFingerprint`, `BinarySymbolIdentity`, physical entry, and implementation body are distinct concepts.

The same semantic specialization may have different ABI realizations under different compilation plans without becoming a different language-level instantiation.

Different semantic specializations may have identical ABI signatures without becoming the same semantic callable.

### 16.3 Native Sec ABI stability

Sec 0.1 does not grant generic specializations a stronger cross-release native ABI stability guarantee than ordinary Sec callables.

Separate-compilation artifacts must verify the compatibility facts required by the owning ABI and artifact rules.

### 16.4 FFI

An unresolved generic extern declaration is invalid.

A concrete generic Sec type is not automatically FFI-compatible merely because its generic arguments are fully substituted.

Foreign ABI legality and representation remain independently governed by the FFI and ABI rulebooks for the active `CompilationPlan`.

### 16.5 Generic wrappers around foreign calls

A generic Sec wrapper may call concrete foreign declarations normally. Monomorphization specializes the Sec wrapper; it does not specialize the foreign provider.

### 16.6 Callback adaptation

A concrete generic callable may be used as the source callable for foreign callback adaptation when, after specialization, it satisfies the same ordinary callback requirements as any other concrete Sec callable.

A callback thunk adapts that concrete callable to the required foreign ABI. It is not a runtime generic function and does not create a general foreign export mechanism.

### 16.7 ABI-hidden versus realization-only parameters

Canonical ABI-hidden parameters, such as hidden return storage, are part of the ABI contract when required by the owning ABI rules.

Realization-only parameters used by a shared internal generic implementation are separate implementation details.

A realization-only parameter must not leak across a declared native or foreign ABI boundary unless the owning ABI independently defines it as part of that boundary.

The plan entry or adapter materializes realization-only facts internally before invoking the shared body.

## 17. Incremental compilation and fingerprints

### 17.1 Identity versus validity

Canonical generic identity and cache validity are separate concepts.

An `InstantiationKey` answers which semantic specialization an entity is.

A fingerprint answers whether a previously computed artifact for that entity may still be reused.

A body edit, dependency edit, compiler semantic change, target change, optimization change, or backend change may invalidate one or more cached artifacts without creating a different language-level instantiation identity.

### 17.2 Cache layers

Incremental implementations should distinguish at least conceptually between:

- generic template artifacts;
- concrete semantic instantiation artifacts;
- semantic analysis artifacts;
- `CompilationPlan`-specific realization artifacts;
- physical implementation-sharing artifacts.

A higher physical layer must never become the source of truth for a lower semantic layer.

### 17.3 Template fingerprints

A generic template fingerprint should be derived from canonical typed semantics and specialization-relevant dependencies rather than raw source-file bytes.

Semantically irrelevant whitespace, comments, or source movement should not force semantic invalidation merely because file bytes changed.

### 17.4 Semantic and provenance fingerprints

Semantic validity and source/debug provenance are separate concerns.

A source-location change may require refreshed diagnostics, LSP mapping, or debug information while leaving semantic specialization reusable.

### 17.5 Dependency-sensitive invalidation

Cached generic results should record the semantic facts on which they depend.

Where practical, dependency invalidation should be fact-oriented rather than treating any edit to a declaration or file as a change to every fact it can provide.

A dirty dependency whose recomputed canonical result is unchanged need not cause unrelated transitive invalidation.

### 17.6 Concrete dependencies

Concrete specialization dependencies are substitution-specific.

A change to `Packet` destruction semantics may invalidate `Foo[Packet]` and its dependents without invalidating `Foo[int32]` when no dependency relation requires it.

### 17.7 Plan changes

Target-independent specialization facts should be reusable across compatible `CompilationPlan` changes.

Target-, ABI-, layout-, optimization-, backend-, or machine-specific artifacts must include the relevant plan and schema compatibility facts.

### 17.8 Compiler versioning

Compiler semantic-rule changes, serialized artifact schema changes, and backend changes are distinct invalidation causes.

A backend-only change should not require semantic re-specialization when semantic and serialization compatibility remain valid.

### 17.9 Cache is not language truth

A missing, corrupt, stale, incompatible, or undecodable cache artifact is a cache miss rather than a change in Sec semantics.

Cold-cache and valid warm-cache compilation of identical canonical inputs must produce equivalent semantic results.

## 18. Termination and resource limits

### 18.1 Compiler termination

Monomorphization must terminate as a compiler process.

For every source program, the compiler must complete specialization or report a semantic failure, a resource-limit failure, or another ordinary compilation failure. It must not recurse or instantiate indefinitely.

### 18.2 Finite canonical graph

The active specialization graph must contain a finite set of canonical `InstantiationKey` values.

Revisiting an existing key does not create another specialization.

### 18.3 Finite cycles are permitted

A cycle containing a finite set of canonical instantiations is not by itself a monomorphization error.

For example:

```text
First[int]
    -> Second[int]
        -> First[int]
```

contains two canonical specializations.

It may correspond to runtime recursion, but it is not unbounded generic specialization.

### 18.4 Runtime recursion and layout recursion are separate

Ordinary runtime recursion is governed by the call graph and relevant resource analyses.

Recursive by-value type representation is governed by layout rules.

A type may have a finite generic instantiation set while still having invalid infinite by-value layout.

### 18.5 Non-converging specialization

A semantic monomorphization failure occurs when the compiler can prove that specialization demand generates an unbounded sequence of distinct canonical instantiations.

For example, a dependency pattern equivalent to:

```text
Expand[T]
    -> Expand[Option[T]]
```

may generate:

```text
Expand[int]
Expand[Option[int]]
Expand[Option[Option[int]]]
...
```

when each step produces a new canonical key and no finite canonical boundary is reached.

### 18.6 Canonicalization precedes recursion comparison

Changing source syntax alone is not sufficient to prove specialization growth.

Generic arguments are resolved and canonicalized before the compiler determines whether a dependency produces a new `InstantiationKey`.

### 18.7 Resource limits are separate

A compiler may impose documented resource limits for robustness, including limits on demanded instantiations, active dependency depth, Semantic IR size, provenance size, or compiler work/memory.

Reaching such a limit is not proof that the semantic instantiation graph is infinite.

If non-convergence has not been proven, the compiler reports a resource-limit failure rather than claiming semantic non-convergence.

Sec 0.1 defines no arbitrary language-level maximum generic instantiation count or depth.

### 18.8 Structural accounting

A structural specialization budget counts canonical demanded entities rather than duplicate requests.

Parallel requests, cache state, source spelling, and physical code sharing must not alter the canonical instantiation count.

### 18.9 No runtime-generic fallback

The compiler must not respond to specialization explosion, non-convergence, or resource exhaustion by silently changing the generic model to type erasure, implicit boxing, runtime dictionaries, or reflection-based generic dispatch.

### 18.10 Tooling budgets

Interactive tooling may use smaller soft budgets for speculative specialization queries.

Tooling budget exhaustion yields an incomplete tooling result, not a persistent semantic declaration that the program is valid or invalid.

Incomplete tooling results must not be reused as positive compilation proof.

## 19. Cost model and compilation policy

### 19.1 Cost domains

Sec distinguishes:

- semantic specialization cost;
- plan-realization compilation cost;
- runtime realization cost.

These are not interchangeable.

### 19.2 Distinct quantities

The following are distinct quantities:

- number of canonical semantic instantiations;
- number of analysis nodes;
- number of plan-specific callable entries;
- number of physical implementation bodies;
- number of binary symbols;
- number of emitted machine-code bytes.

### 19.3 No fixed compilation-complexity guarantee

Sec does not define a fixed asymptotic compiler-time guarantee for generic programs.

A finite generic program may still require substantial compiler work and may encounter implementation resource limits as defined in section 18.

### 19.4 No normative zero-cost claim

This rulebook does not define Sec generics as universally "zero-cost".

Sec guarantees that generic semantics do not require a runtime-generic representation.

A compiler may nevertheless choose a shared concrete implementation that carries a small already-resolved runtime cost, such as passing a concrete size or selected destructor address, when doing so is profitable and preserves all applicable semantics.

### 19.5 Size-oriented policy

A size-oriented plan may prefer:

- shared implementation bodies;
- shared helpers;
- reduced inlining;
- small adapters;
- concrete realization parameters that reduce code duplication.

### 19.6 Performance-oriented policy

A performance-oriented plan may prefer:

- distinct physical specialization;
- direct calls;
- constant propagation;
- specialized loops;
- vectorization;
- aggressive inlining;
- elimination of realization-only parameters.

### 19.7 Debugging and profiling policy

A debugging or profiling plan may preserve distinct entries or thunks for attribution while sharing an internal implementation body.

### 19.8 Semantic contracts dominate profitability

Implementation sharing or code-size optimization must not introduce:

- hidden allocation that violates allocation semantics or guarantees;
- implicit boxing;
- changed ownership;
- merged static state;
- weakened effects;
- invalid ISR behavior;
- ABI incompatibility;
- violated stack or other active resource contracts.

### 19.9 Build resource budgets

A project or target profile may impose physical code, ROM, RAM, stack, or other resource budgets according to its owning rules.

Failure to satisfy such a budget is a `CompilationPlan` or build failure for that selected plan, not evidence that the generic template is invalid or that the specialization graph is non-converging.

Generic compiler resource limits and final binary resource budgets are distinct failure classes.

## 20. Diagnostics and tooling

### 20.1 User-facing identity

Diagnostics and Sec-aware tooling identify a concrete specialization using canonical Sec-level identity such as:

```text
Foo[Packet]
```

Internal cache identifiers, fingerprints, mangled symbols, sharing classes, thunks, adapters, or machine addresses are not normal source-level identity.

### 20.2 Template errors

A template-independent semantic error is reported at the generic declaration and is not repeated once per possible specialization.

### 20.3 Concrete request failures

A failure caused by a concrete request, such as unsatisfied constraints or incompatible explicit generic arguments, uses the requesting source location as the primary diagnostic where appropriate and identifies the canonical concrete substitution.

### 20.4 Instantiation traces

A nested concrete failure must preserve enough causal provenance to produce a finite deterministic instantiation trace.

For example:

```text
while instantiating:
    Outer[Packet]
    -> Middle[Packet]
    -> Inner[Packet]
```

Compiler-generated dependencies should identify the source-level operation that required them when this improves the diagnostic.

### 20.5 Cycles and long traces

Instantiation traces are cycle-aware and finite.

Presentation may abbreviate a very long trace, but the compiler retains the causal provenance needed for diagnostics and inspection.

### 20.6 Context failures

A semantically valid concrete specialization may still be invalid in one `CompilationPlan` or execution context.

Diagnostics must distinguish this from invalid generic template semantics.

For example, `Process[Packet]` may be a valid specialization but be forbidden in an ISR because its concrete stack or effect requirements violate the ISR contract.

### 20.7 LSP

Definition navigation from a concrete generic use normally targets the source generic declaration.

Hover, signature help, analysis views, or explicit specialization inspection may additionally show:

- resolved concrete generic arguments;
- whether arguments were explicit or inferred;
- the concrete semantic signature;
- concrete analysis facts;
- specialization provenance.

### 20.8 AST and Semantic IR inspection

Source AST represents the generic syntax actually written by the programmer.

Inferred arguments are semantic-resolution facts and need not be rewritten into source AST.

Template Semantic IR may contain generic symbols. Runtime-relevant concrete Semantic IR must expose the concrete substitution and satisfy the generic closure rules in section 13.

### 20.9 Physical implementation inspection

Explicit compiler-inspection tooling may expose:

- `GenericTemplate`;
- `InstantiationIdentity`;
- `PlanRealization`;
- binary entry;
- implementation body;
- sharing relationships;
- hidden realization parameters;
- cache and fingerprint information.

Exposing these compiler layers for inspection does not make them Sec language semantics.

### 20.10 Debugging and profiling

Distinct semantic specializations retain distinct debug and profiling identities where the tooling model can represent them, even when machine code is shared.

Machine address is not canonical Sec callable identity.

Any future source-level callable equality or raw function-address semantics are defined by their owning rulebooks rather than inferred from monomorphization.

## 21. Determinism and reproducibility

Canonical generic behavior must be deterministic for identical relevant compilation inputs.

The following must not change semantic generic results merely because of implementation order:

- thread scheduling;
- worker completion order;
- hash-map iteration order;
- filesystem enumeration order;
- module request order;
- cache hit order;
- which requester first discovered an instantiation;
- which specialization first became a physical-sharing candidate.

Canonical instantiation identity, request deduplication, semantic specialization, representative diagnostic traces, structural resource accounting, binary identities, and physical-sharing representative selection must use deterministic inputs and ordering.

A cold build and a valid warm-cache build with identical semantic inputs must produce equivalent semantic results.

Different compiler versions or optimization implementations may choose different legal physical realization strategies without changing canonical Sec generic semantics.

## 22. Required compiler invariants

The compiler must preserve all of the following invariants:

1. One canonical declaration plus one ordered canonical complete generic argument set identifies one canonical semantic specialization for the applicable specialization role.
2. Nominally different arguments remain semantically distinct even when physical representation matches.
3. Generic templates are validated under declared constraints before specialization.
4. Concrete specialization is demand-driven.
5. Repeated canonical demand reuses one semantic instantiation.
6. Requesting module identity does not create a second semantic specialization.
7. Substitution is simultaneous and semantic rather than textual.
8. Generic-dependent semantic choices are fully resolved before representation-dependent lowering.
9. Generic closure does not require premature target layout or ABI closure.
10. Physical implementation sharing occurs only after semantic specialization is known sufficiently to prove sharing safe.
11. Physical sharing never merges per-instantiation static state or semantic analysis identity.
12. Realization-only parameters carry already-resolved concrete facts and never defer ordinary generic semantic resolution to runtime.
13. Generic templates do not acquire unresolved runtime ABIs.
14. A concrete generic type is not automatically FFI-compatible.
15. Cache validity and semantic identity remain separate.
16. Cache or tooling presence never creates program demand or LinkRoots.
17. Finite canonical instantiation cycles are not automatically monomorphization errors.
18. Proven unbounded generation of distinct canonical instantiations is a semantic monomorphization failure.
19. Compiler resource exhaustion is not proof of semantic non-convergence.
20. Optional optimization does not establish generic language validity.
21. The compiler does not fall back silently to erased or dictionary-based runtime generics.
22. Generic results and diagnostics are deterministic for identical canonical inputs.

## 23. Required tests

At minimum, conformance and implementation tests must cover the following families.

### 23.1 Identity

- repeated identical generic type requests reuse one canonical identity;
- repeated identical generic callable requests reuse one canonical identity;
- explicit and inferred argument resolution converge on the same canonical specialization;
- ordered generic arguments produce distinct identities when their order differs;
- nominally distinct representation-identical arguments remain distinct specializations;
- phantom generic enum arguments remain part of nominal identity;
- generic method identity distinguishes enclosing and method-level arguments correctly.

### 23.2 Template validity

- a generic body using an operation not guaranteed by its constraints is rejected at template level;
- constraint-mediated operations are accepted when the declared contract guarantees them;
- specialization does not perform arbitrary new overload hunting;
- non-dependent semantic references remain stable across specializations.

### 23.3 Demand

- unused concrete generic functions and methods do not become active program instantiations merely because templates exist;
- repeated requests from multiple modules reuse the same semantic specialization;
- tooling-created or cached specializations do not become LinkRoots;
- dead cached specializations do not materialize static storage or machine code.

### 23.4 Substitution and concrete semantics

- nested generic types are recursively concretized;
- generic impl owner parameters substitute correctly;
- method-level parameters and enclosing type parameters substitute in their correct semantic roles;
- generic enums preserve concrete owner/member identity;
- concrete callable signatures contain no unresolved generic parameters;
- borrowed, forced-consuming, and typed variadic parameter shapes are preserved through specialization;
- concrete associated/static references bind the correct specialization identity.

### 23.5 Ownership and destruction

- copyable and non-copyable substitutions receive correct concrete ownership behavior;
- a template cannot depend on copyability without a generic contract that guarantees it;
- concrete trivial destruction may be optimized away only after semantic resolution;
- per-instantiation static state remains distinct under shared executable code.

### 23.6 Analysis

- concrete generic specializations appear as distinct semantic call-graph nodes;
- physical body sharing does not merge concrete effect or stack analysis identity;
- a concrete specialization may refine an inferred effect summary without weakening a declared guarantee;
- one specialization reachable from an ISR does not make every specialization sharing its body ISR-reachable.

### 23.7 Generic closure

- runtime-relevant concrete IR contains no unresolved generic parameter in signatures, stored members, nested generic arguments, ownership operations, destruction, or static identity;
- target-sized concrete types remain valid concrete generic arguments before target layout is physically resolved;
- generic closure succeeds independently of optional physical layout facts not yet required;
- unsupported lowering rejects explicitly rather than approximating ownership or destruction semantics.

### 23.8 Sharing

- representation equality alone does not merge semantic specializations;
- legal specializations may share one implementation body;
- distinct ABI entries may converge on one shared internal body;
- a shared body may receive a concrete size, destructor address, operation address, or static-storage address without introducing runtime generic lookup;
- a size-oriented plan may share implementations that a performance-oriented plan physically specializes;
- disabling physical sharing does not change semantic instantiation count.

### 23.9 Linking and ABI

- a canonical semantic instantiation may exist without a binary symbol;
- identical specialization requests across separate compilation units coalesce or otherwise resolve without semantic duplication;
- native symbol identity preserves specialization identity where required;
- one semantic specialization may have different ABI fingerprints under different plans;
- representation-identical distinct specializations may have equal ABI signatures without becoming the same semantic callable;
- realization-only hidden parameters do not alter externally required ABI entries.

### 23.10 FFI

- unresolved generic extern declarations are rejected;
- concrete generic types are independently validated for foreign ABI compatibility;
- a valid concrete generic callable may participate in ordinary callback adaptation when callback requirements are satisfied;
- callback adaptation does not create runtime generic dispatch.

### 23.11 Incremental compilation

- semantic source-equivalent edits can preserve semantic specialization fingerprints while refreshing provenance;
- concrete argument-specific changes invalidate only dependent specializations where dependency tracking permits;
- target changes invalidate plan-specific realization while preserving compatible target-independent specialization facts;
- optimization-profile changes may alter sharing without altering semantic instantiation identity;
- warm and cold cache paths produce equivalent semantic results.

### 23.12 Termination

- a self-recursive concrete generic function with a stable key produces one specialization and ordinary runtime recursion;
- mutually recursive stable specializations produce a finite canonical cycle;
- direct recursive by-value storage is reported by layout semantics rather than as generic instantiation explosion;
- valid recursion through finite indirection remains accepted;
- changing-argument unbounded specialization is rejected when non-convergence is proven;
- syntactically changing arguments that canonicalize back to an existing key do not produce false non-convergence;
- an artificial compiler instantiation limit produces a resource-limit diagnostic rather than a semantic non-convergence diagnostic;
- parallel duplicate requests count as one canonical instantiation for structural accounting.

### 23.13 Diagnostics and determinism

- template-independent errors are not repeated for every concrete requester;
- concrete request failures identify the concrete substitution and source request;
- nested failures produce finite deterministic instantiation traces;
- cache state and worker order do not change representative trace selection;
- internal shared-body names do not replace canonical Sec-level identities in normal diagnostics.

## 24. Summary

Sec monomorphization is canonical compile-time semantic specialization.

A valid generic template plus one canonical complete substitution creates or reuses one distinct semantic specialization. Semantic specialization is demand-driven, deterministic, independent of physical representation, and complete before runtime-relevant generic semantics cross into representation-dependent lowering.

Distinct semantic specializations may share physical implementation when all applicable semantics and `CompilationPlan` contracts remain preserved. Sharing may use minimal already-resolved realization parameters, but it does not introduce runtime generic type meaning or a universal runtime generic environment.

Generic identity, analysis identity, plan realization, ABI identity, binary symbol identity, machine entry, and implementation body remain separate compiler concepts. This separation permits aggressive implementation reuse, precise diagnostics, incremental compilation, separate compilation, and target-specific realization without weakening Sec's static generic model.
