# MD-016 — Canonical declarations for shaped metadata (part 1)

- **Status:** Applied 2026-10-18 (owner-provided) — synced into shaped-types.md rev 2.1 (§§ 4.1, 6.0, 7.1, 15.3, 29.3), compiler_known_members.md ("Shaped metadata families") and core-library.md § 24; MD-016 stays open for the remaining families (§ 12.5). No contradiction with the current owner text was found (§ 13.3): § 7 "additional categories" was read as future versioning and § 29.3 "at least" was closed to the nine variants. Owner decisions while applying: `ShapeError` carries the `error` marker and is declared once in `sec/core/error.sec`. Frontend implemented 2026-10-18 (literals, members, indexed reads, initializers, TensorLayout enum, ShapeError; TestShapedMetadataSurface); Semantic IR, LSP declarations and consuming operations remain open in governance `frontend.shaped-types`.
- **Created:** 2026-10-18
- **Updated:** 2026-10-18
- **Revision:** 1.0
- **Sec version:** 0.1
- **Canonical correction path:** `rules/corrections/applied/md016-shaped-metadata-declarations-correction-20261018.md`
- **Amends:** `rules/collections/shaped-types.md` §§4–7, 15, 16, 25, 29, 31, 33–34; `rules/compiler/compiler_known_members.md` shaped registry; `rules/library/core-library.md` shaped/compiler boundary
- **Conforms to:** `rules/declarations/generics.md`, `rules/declarations/impl.md`, `rules/declarations/properties.md`; current numeric-`Len` and compiler-known documentation decisions
- **Replaces:** No existing numbered paragraphs; append and cross-reference without renumbering established rules
- **Scope:** Complete declaration and checking contract of `Shape[Rank]`, `Strides[Rank]`, `TensorLayout[Rank]`, `Axes[Rank]`, `AxisList[Count]` and their required `ShapeError` cases. This is the first implementation-ready portion of MD-016, not a claim to close the whole MD.

## 1. Authority and exact boundary

**§1.1.** The five names in this document are public, compiler-known, **nominal Sec types**. They are not aliases of arrays, tuples or generic containers. Rank and Count are compiler-known **compile-time non-negative integer arguments** of these particular type forms. They do **not** authorize arbitrary integer/const generic parameters in ordinary user-defined types.

**§1.2.** The public surface below consists exactly of the declared `init` members, read-only properties, enum variants and compiler-known indexed expressions described in §§2–6. Private fields in the descriptive source definitions are implementation detail and are not exposed as public members. These declarations are *real source-facing contracts*; a compiler intrinsic or optimized physical representation does not excuse an incomplete LSP or documentation signature.

**§1.3.** All five types are immutable metadata values. They contain no owned backing storage, cannot prolong a shaped view's backing lifetime, and are copyable when instantiated with valid compile-time cardinality. Reads never implicitly allocate. There is no public writable element access, `.Ptr`, `.Strides`, `.Layout`, `.MemorySpace`, `.IsContiguous` or shaped-payload `.SizeOf` on these metadata types merely because they contain multiple components.

**§1.4.** All numeric indexes of metadata values are `uint`. Subscript bounds follow the normal safe Sec indexed-read rules: a provably invalid constant index is diagnosed at compile time; a runtime out-of-bounds read fails under the language's indexed-access bounds contract. Do not change a normal bounds failure into a fabricated `Result` return. An indexed read yields the scalar value, not a mutable borrow of an internal component.

**§1.5.** The existing compiler-known positional literal forms (`Shape[3] { 2, 4, 8 }`, `Axes[3] { 2, 0, 1 }`, `AxisList[2] { 1, 3 }`) are maintained. This does not introduce general positional struct literal construction for ordinary user structs. Each literal is typed using the corresponding exact component type and arity below.

## 2. `Shape[Rank]`

**§2.1.** `Shape[Rank]` contains *exactly Rank unsigned extents in source-axis order*. A zero extent is valid. It is a nominal value, not a `uint[Rank]` alias. Component values do not become signed by implicit conversion.

**§2.2.** The complete public Sec-shaped declaration for this type is:

```sec
/**
 * Describes the unsigned extents of a shaped value.
 *
 * Description:
 *   Rank is a compiler-known non-negative integer argument. The value
 *   contains exactly Rank extents in source-axis order; zero is permitted.
 *   Shape[Rank] has nominal identity and independently owned metadata.
 */
type Shape[Rank] struct {
    // Private compiler-known representation; not a public field.
    _extents: uint[Rank],
}

impl Shape[Rank] {
    /**
     * Constructs a shape from exactly Rank extents.
     *
     * Parameters:
     *   - extents: Fixed-size unsigned extents, in source-axis order.
     *
     * Returns:
     *   The fully initialized Shape[Rank] value.
     *
     * Errors:
     *   None. Storing extents alone does not materialize storage or compute
     *   an overflowing total element count.
     */
    init(extents: uint[Rank]) {
        self._extents = extents
    }

    /**
     * Gets the number of axes described by this shape.
     *
     * Returns:
     *   The compile-time Rank as uint.
     */
    property Rank: uint {
        get {
            return Rank
        }
    }
}
```

**§2.3.** In addition to the ordinary constructor, `Shape[Rank] { e0, ..., eN }` is the canonical compiler-known literal. It requires exactly Rank source expressions of type `uint` or contextually representable unsigned integer literals; it creates an independently owned metadata value. For dynamic extents, the literal is **infallible**, as is `new Shape[Rank](extents)`. Do not require `try` merely for assembling extents.

**§2.4.** The compiler-known indexed expression `shape[axis]` accepts `axis: uint` and produces `uint`. It has no writable counterpart. Its source-type contract is `Shape[Rank] × uint -> uint`, with `0 <= axis < Rank`. The compiler must expose the resolved index operation in semantic information and LSP; it is indexed expression syntax, **not** an invented public `.At()` function.

**§2.5.** Computing the *product* of extents is **not part of ordinary `Shape` construction**. When a consumer requires a representable total count (`value.Len`, backing construction, reshape proof, checked materialization), multiplication is validated in the consumer's context, using `ShapeError.ElementCountOverflow` where that consumer is fallible. Static overflow is diagnosed instead of generating an unusable shape-backed value. This preserves the independent usefulness of a shape descriptor and avoids a needless fallible constructor.

**§2.6.** No implicit conversion exists between `Shape[Rank]`, `uint[Rank]`, `list[uint]` or a different `Shape[OtherRank]`. The explicit lifecycle initializer is not a general coercion rule.

## 3. `Strides[Rank]`

**§3.1.** `Strides[Rank]` contains exactly Rank **signed element strides**, not byte offsets. Negative strides denote reverse traversal; zero stride may denote broadcast reuse. Neither is intrinsically invalid. Creating a standalone descriptor grants no unsafe storage authority.

**§3.2.** Complete declaration:

```sec
/**
 * Describes signed element strides for an affine-shaped mapping.
 *
 * Description:
 *   Each stride counts elements, not bytes. Negative, zero and positive
 *   strides are valid metadata. A backing range must still be validated
 *   before safe indexed access through a shaped view.
 */
type Strides[Rank] struct {
    // Private compiler-known representation; not a public field.
    _strides: int[Rank],
}

impl Strides[Rank] {
    /**
     * Constructs a stride descriptor from exactly Rank element strides.
     *
     * Parameters:
     *   - strides: Fixed-size signed strides, in source-axis order.
     *
     * Returns:
     *   The initialized immutable Strides[Rank] value.
     *
     * Errors:
     *   None. Descriptor creation does not validate a particular backing.
     */
    init(strides: int[Rank]) {
        self._strides = strides
    }

    /**
     * Gets the number of axes represented by the descriptor.
     *
     * Returns:
     *   The compile-time Rank as uint.
     */
    property Rank: uint {
        get {
            return Rank
        }
    }
}
```

**§3.3.** `strides[axis]` is a compiler-known **read-only** indexed expression `Strides[Rank] × uint -> int`, with the same index bounds as §2.4. A positional literal `Strides[Rank] { s0, ..., sN }` is permitted as the matching compiler-known descriptor literal, with exactly Rank signed stride values. There is no implicit conversion between an element stride and a byte stride.

**§3.4.** The compiler must not reject `Strides[3] { -1, 0, 4 }` as a metadata value. A later attempt to use any strides with concrete backing remains subject to checked origin, span, overlap, alias, alignment and `MemorySpace` rules in `shaped-types.md` §§6, 17–20 and `storage.md`.

## 4. `TensorLayout[Rank]`

**§4.1.** `TensorLayout[Rank]` is a generic nominal **enum**. A concrete Rank is part of type identity, even though the enum tag's physical representation need not store Rank.

**§4.2.** The complete closed Sec declaration for Sec 0.1 is:

```sec
/**
 * Classifies the public affine layout of a Rank-axis shaped value.
 *
 * Description:
 *   This enum describes the source-visible layout contract, not the
 *   backend's private tiled, sparse, device-specific or fused encoding.
 */
enum TensorLayout[Rank] {
    // Forward contiguous dense traversal with the last axis fastest.
    DenseRowMajor

    // Forward contiguous dense traversal with the first axis fastest.
    DenseColumnMajor

    // An affine mapping not classified as either canonical dense order.
    Strided
}
```

**§4.3.** There are exactly three public variants in this version. The phrase "additional categories may be defined" in the older shaped rulebook is a **future versioning statement**, not permission for individual compilers to invent extra public cases in Sec 0.1. Backend-specific layout variants remain internal until a separately approved additive language revision.

**§4.4.** The source forms are `TensorLayout[Rank].DenseRowMajor`, `.DenseColumnMajor`, and `.Strided`. No value mutation, `.Ptr`, invented default `init()`, or dynamically indexed element API is provided. Consumers that require a particular layout validate against the explicit enum value.

**§4.5.** For degenerate shapes (including zero or unit extents), the compiler selects a deterministic canonical classification consistent with the owner's layout contract; it may not report different categories for observably identical metadata between Sema and LSP. This rule does not require a physical allocation to answer `.Layout`.

## 5. `Axes[Rank]`

**§5.1.** `Axes[Rank]` contains exactly Rank indices and is a complete permutation of `0..<Rank`. Duplicates and out-of-range values are invalid. A permutation of rank zero is the unique empty permutation where rank-zero metadata is otherwise admitted by a consuming operation; it does not introduce rank-zero shaped storage by itself.

**§5.2.** Complete declaration:

```sec
/**
 * Describes a complete permutation of all axes of a Rank-axis value.
 *
 * Description:
 *   The sequence contains every index in 0..<Rank exactly once.
 *   Order determines the destination-axis order.
 */
type Axes[Rank] struct {
    // Private, invariant-protected representation.
    _axes: uint[Rank],
}

impl Axes[Rank] {
    /**
     * Validates and constructs a complete axis permutation.
     *
     * Parameters:
     *   - axes: Exactly Rank axis indices, in destination-axis order.
     *
     * Errors:
     *   - ShapeError.InvalidAxis: An index is not in 0..<Rank.
     *   - ShapeError.DuplicateAxis: An index appears more than once.
     *
     * Description:
     *   This fallible initializer establishes the complete permutation
     *   invariant before the value becomes observable.
     */
    init(axes: uint[Rank]) ShapeError {
        let mut i: uint := 0
        while i < Rank {
            if axes[i] >= Rank {
                return Err(ShapeError.InvalidAxis)
            }

            let mut j: uint := 0
            while j < i {
                if axes[j] == axes[i] {
                    return Err(ShapeError.DuplicateAxis)
                }
                j += 1
            }
            i += 1
        }
        self._axes = axes
    }

    /**
     * Gets the number of axes in this complete permutation.
     *
     * Returns:
     *   The compile-time Rank as uint.
     */
    property Rank: uint {
        get {
            return Rank
        }
    }
}
```

**§5.3.** The initializer above is a **source-facing signature and complete reference algorithm**: on duplicate detection it returns `ShapeError.DuplicateAxis`; construction commits only after all checks. The compiler may optimize the loops, but must preserve exactly their ordering, error results and atomicity. No placeholder public members or unspecified extra constructor overloads exist.

**§5.4.** The canonical positional literal `Axes[Rank] { a0, ..., aN }` remains valid when all entries and the permutation invariant are statically proven. A statically invalid literal produces an actionable compile-time diagnostic naming the bad or duplicated axis. For runtime-supplied values use `try new Axes[Rank](axes)` and handle `ShapeError`; the compiler must not silently panic or invent a `Result`-typed literal.

**§5.5.** `axes[position]` is a compiler-known read-only indexed expression `Axes[Rank] × uint -> uint`, where `0 <= position < Rank`. It returns the source-axis index selected for that destination position. No mutable element access can break the permutation invariant.

**§5.6.** `Permute(axes: Axes[Rank])` is a **separate** view-transforming operation. Axis validation must be completed when constructing `Axes`; a valid `Axes[Rank]` never needs a second uniqueness/range check against its *own* rank merely because it is passed to `Permute`.

## 6. `AxisList[Count]`

**§6.1.** `AxisList[Count]` is an immutable, nominal, ordered sequence of exactly Count **distinct** `uint` indices. Unlike `Axes[Rank]`, it need not list every axis of a source. Its Count does not encode the rank of its eventual consuming tensor.

**§6.2.** Complete declaration:

```sec
/**
 * Describes an ordered selection of distinct source-axis indices.
 *
 * Description:
 *   Count fixes the number of selected axes, not the rank of the
 *   source value. The consuming operation checks indices against its
 *   source rank. Axis order is semantically significant.
 */
type AxisList[Count] struct {
    // Private, invariant-protected representation.
    _axes: uint[Count],
}

impl AxisList[Count] {
    /**
     * Validates and constructs an ordered selection of unique axes.
     *
     * Parameters:
     *   - axes: Exactly Count distinct unsigned axis indices.
     *
     * Errors:
     *   - ShapeError.DuplicateAxis: An axis index occurs more than once.
     *
     * Description:
     *   No source rank is supplied here. Domain validation against a
     *   source tensor is performed by the consuming operation.
     */
    init(axes: uint[Count]) ShapeError {
        let mut i: uint := 0
        while i < Count {
            let mut j: uint := 0
            while j < i {
                if axes[j] == axes[i] {
                    return Err(ShapeError.DuplicateAxis)
                }
                j += 1
            }
            i += 1
        }
        self._axes = axes
    }

    /**
     * Gets the number of selected axes.
     *
     * Returns:
     *   The compile-time Count as uint.
     */
    property Count: uint {
        get {
            return Count
        }
    }
}
```

**§6.3.** `AxisList[Count] { a0, ..., aN }` is a statically checked compiler-known literal; duplicates are compile-time errors. Runtime values use `try new AxisList[Count](axes)` and receive `ShapeError.DuplicateAxis` when uniqueness fails. The array length is statically Count for both constructions.

**§6.4.** `axisList[position]` is a compiler-known read-only indexed expression `AxisList[Count] × uint -> uint`, with `0 <= position < Count`. It returns the selected source-axis index at that position; it is not a mutable reference.

**§6.5.** `AxisList[2] { 1, 3 }` is a valid standalone descriptor even though `3` might be out of range for a later rank-three tensor. The **consumer**, not `AxisList` construction, must reject that combination as `ShapeError.InvalidAxis`. This distinction is mandatory for generic `Contract` use, where two source tensors may have different ranks.

## 7. Closed public error contract used by these constructors

**§7.1.** `Axes` and `AxisList` use the existing public `ShapeError` family. This correction proposes making the nine already named cases in `shaped-types.md` §29.3 the **exhaustive Sec 0.1** variant set; no invisible compiler-private cases may escape into a public `ShapeError` value.

```sec
/**
 * Reports a failed runtime shape, axis, or affine validation.
 *
 * Description:
 *   These are the complete source-visible ShapeError cases in Sec 0.1.
 *   An implementation may preserve more detailed internal causes in
 *   diagnostics, but must not expose undeclared enum variants.
 */
enum ShapeError {
    // The number of axes is incompatible with the required operation.
    RankMismatch

    // A required dimension or corresponding extent is incompatible.
    DimensionMismatch

    // A required total logical element count cannot fit in uint.
    ElementCountOverflow

    // A reshape cannot preserve the required logical shape and backing.
    InvalidReshape

    // The requested broadcast mapping is incompatible with its source.
    InvalidBroadcast

    // Tensor contraction axes or corresponding extents are incompatible.
    InvalidContraction

    // An axis index is invalid for the required source rank.
    InvalidAxis

    // An axis selection repeats an index that must be unique.
    DuplicateAxis

    // A step value or mapping step is invalid (including zero step).
    InvalidStep
}
```

**§7.2.** `ShapeError` is a nominal ordinary enum with no per-variant payload in this revision. Diagnostics must include *context* (axis index, rank, conflicting dimensions) separately, not pretend these variants have payloads. If the canonical error owner in current uncommitted work differs, reconcile it explicitly **before** applying this §7 rather than silently creating a second public `ShapeError`.

**§7.3.** An error returned by `init` is a **construction error channel** under `impl.md` §15, not a return of `Result[Axes[Rank], ShapeError]` from a normal function named `New` and not an explicit `Result` success payload in the signature.

## 8. Complete public member availability table

**§8.1.** For this part of MD-016, the **entire new public surface** is:

| Source type | Public members | Compiler-known indexed read | Construction |
| --- | --- | --- | --- |
| `Shape[Rank]` | `Rank: uint` get-only | `[axis: uint]` yields `uint` | `init(extents: uint[Rank])` infallible; `Shape[Rank] { ... }` |
| `Strides[Rank]` | `Rank: uint` get-only | `[axis: uint]` yields `int` | `init(strides: int[Rank])` infallible; `Strides[Rank] { ... }` |
| `TensorLayout[Rank]` | `DenseRowMajor`, `DenseColumnMajor`, `Strided` | None | Enum variants; no `init` |
| `Axes[Rank]` | `Rank: uint` get-only | `[position: uint]` yields `uint` | `init(axes: uint[Rank]) ShapeError`; valid static literal |
| `AxisList[Count]` | `Count: uint` get-only | `[position: uint]` yields `uint` | `init(axes: uint[Count]) ShapeError`; valid static literal |

**§8.2.** No implicit additional constructors, iterators, mutable accessors, cast operators, pointer members, public fields, or `ToString` overloads are created by this correction. In particular, neither `.Len` nor `.SizeOf` is automatically a shaped-payload property on a descriptor: the rank/count property is authoritative for descriptor cardinality. Ordinary compiler-known physical type-layout queries remain governed by the existing layout rulebook.

**§8.3.** The private array backing shown above is a valid *semantic model* of each metadata value, not a mandate to materialize a fixed array for a compile-time constant or to break optimized representation. A generated synthetic definition must still show the actual declared public API above, not an unannotated opaque alias.

## 9. Required typing and validation order

**§9.1.** Resolve the compiler-known Rank/Count argument and type identity; validate that it is a compile-time integer in the type family's supported domain. This must happen before materialization, generic instantiation of dependent operations or target lowering.

**§9.2.** Check constructor/literal **arity** and exact element types, then check invariants. Compile-time-proven invalid literals are compile errors, not runtime `Err`; dynamically checked constructors use the stated `ShapeError` channel. Never allow caller mutation of private descriptor components to bypass validation.

**§9.3.** `Axes` checks that each component is in `0..<Rank` and that each appears exactly once. `AxisList` checks uniqueness only, and the relevant operation checks each component against its own source rank.

**§9.4.** For signed strides, use checked intermediate arithmetic for any computed address mapping; metadata may contain negative or zero strides without granting addressability or mutable access through overlapping views.

**§9.5.** A syntactically valid metadata value does not prove existence of backing storage, element address stability, safe `.Ptr`, absence of aliasing, or layout feasibility in a target-specific memory space.

## 10. Compiler, Semantic IR and LSP obligations

**§10.1.** Register each generic nominal family and each available public member in the existing typed compiler-known registry, not an LSP-specific table. The `ShapeError` type and variants resolve through the normal nominal enum registry.

**§10.2.** Semantic IR preserves which descriptor was constructed, its compile-time cardinality, each runtime component if any, static validity proofs, checked-init errors, index read semantics and whether the value is independently materialized. MLIR lowering cannot turn a failed checked constructor into success or erase required validation.

**§10.3.** LSP hover and signature help must show the above **exact Sec declarations**, per-member structured comments, and the fallible initializer error channel. For checked construction, show `constructs: Axes[Rank]` / `construction error: ShapeError`, not a fabricated `Result` return type.

**§10.4.** A source hover over `shape[i]`, `strides[i]`, `axes[i]`, or `axisList[i]` must identify the indexed component type and read-only nature. Go-to-definition may navigate to a generated read-only compiler-known definition but must never falsely offer a user-editable generated body as the authoritative implementation.

**§10.5.** Formatters preserve `Shape[Rank] { ... }`, `Axes[Rank] { ... }`, `AxisList[Count] { ... }`, `Strides[Rank] { ... }` and valid `new` constructor syntax; an ordinary user-defined positional-struct-literal rule must not be inferred from these special forms.

## 11. Required positive and negative conformance tests

**§11.1. Positive:** Construct `Shape[3] { 2, 0, 4 }`; confirm Rank=3 and ordered components, independent copyability and zero extent preservation. Construct a shape using runtime extents without an unnecessary `try`.

**§11.2. Positive:** Construct `Strides[3] { -1, 0, 4 }`; confirm signed indexed component types and no rejection at metadata construction. Confirm that subsequent backing validation remains separate.

**§11.3. Positive:** Construct and compare all three `TensorLayout[3]` members; ensure nominal type distinction from `TensorLayout[2]`. Exercise exhaustive `match` of exactly three variants.

**§11.4. Positive:** Construct `Axes[3] { 2, 0, 1 }` as a static literal; construct an equivalent runtime permutation via `try new Axes[3](values)`; verify valid `Permute` order, checked runtime errors, and absence of mutable field access.

**§11.5. Positive:** Construct `AxisList[2] { 1, 3 }` and pair it with a rank-four `Contract` source; verify selected axes and preservation of caller-provided order. Verify the same standalone descriptor is rejected *when consumed* against a rank-three source.

**§11.6. Negative:** Wrong literal cardinality; mismatched signed/unsigned component types; attempted mutation by subscript; out-of-range constant index; duplicate or missing `Axes` component; duplicate `AxisList` component; unguarded fallible `new`; unsafely coerced `Shape[2]` to `Shape[3]`; nonexistent `.Len` or `.Ptr` members on metadata; invented fourth `TensorLayout` variant.

**§11.7. Negative runtime:** Out-of-range dynamic `Axes` value returns `ShapeError.InvalidAxis`; repeated dynamic `Axes`/`AxisList` indices return `ShapeError.DuplicateAxis`; failed constructor does not leak a partially valid value; dynamic indexed reads obey ordinary bounds failure semantics.

**§11.8. Differential:** For every metadata program above, compare Semantic IR and cold compilation, incremental compilation, LSP signatures and chosen target lowering. Test rank-0 metadata where admitted, zero extents, target-sized integer boundaries, multiple concurrent generic ranks, and cross-module use.

## 12. Codex / Claude Code integration instructions

**§12.1.** Apply this slice into existing owners by appending or cross-referencing these numbered contracts **without renumbering** the established sections. Incorporate the complete declarations into `rules/collections/shaped-types.md` near §§4, 6, 7 and 15. Ensure any named type/error declaration has exactly one canonical owner, not duplicated incompatible declarations.

**§12.2.** Update `rules/compiler/compiler_known_members.md` with all five families, their concrete property and constructor identities, compile-time cardinality behavior, indexed reads, and exact error channels. Update `rules/library/core-library.md` boundary notes only as needed; compiler-known intrinsic existence must not depend on stub core helpers.

**§12.3.** Implement/check in order: parser/literal typing → Sema nominal identities and checked validation → Semantic IR → backend lowering → LSP synthetic definitions/hover/completion → positive and negative tests. A rejected compile-time literal must not become a runtime fallback. Unsupported feature diagnostics must say what is missing, not silently reinterpret the type as an ordinary array.

**§12.4.** Synchronize current per-book governance fragments under `governance/` using stable IDs (suggested family: `collections.shaped-types.md016.shape`, `.strides`, `.tensor-layout`, `.axes`, `.axis-list`, `.shape-error`). Merge into current entries; do not duplicate entries already covering these facts. Record separate implemented, partial, unimplemented and identified bug states only after source/tests are verified.

**§12.5.** Keep `MD-016` open: the remaining canonical shaped **owning families**, view type, storage request types, errors outside `ShapeError`, and their operations still require separate exact declaration work. This file is an actual specified implementation slice, not an audit checklist substituting for later type declarations.

## 13. Compatibility and explicit adoption notes

**§13.1.** The following reflect already published shaped semantics: nominal supporting types, fixed Rank/Count, unsigned extents, signed element strides, three layout categories, full-permutation `Axes`, distinct ordered `AxisList`, preserving compiler-known positional literals, the absence of hidden allocation, and source-visible hover requirements.

**§13.2.** The following are **precise completions proposed by this correction**, rather than claims that prior rules explicitly fixed these source members: private descriptor field layout shown as valid Sec source, get-only `.Rank`/`.Count` members, public read-only indexed-expression typing for all four descriptor structs, `Strides` positional literals, using fallible `init(... ) ShapeError` and `try new` for runtime permutation/axis-list creation, and closing the public `ShapeError` to the existing nine named variants. These provisions must be explicitly approved as part of adopting this correction; they must not be silently presented as previously decided.

**§13.3.** The existing publicly cached GitHub page shows `shaped-types.md` revision 2.0 and `generics.md` revision 2.1. The current `main` HEAD could not be freshly verified, and local unpublished rulebooks may supersede parts of the cached pages. Before applying this file, check the live current owner material and reconcile exact contradictions; do **not** let Codex or Claude Code invent a substitute policy where a current owner already defines it.
