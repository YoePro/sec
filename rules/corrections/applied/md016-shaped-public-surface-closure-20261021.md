# MD-016 — Compiler-known declarations: shaped type closure and synchronization

- **Status:** Applied 2026-10-21 — normative integration: `rules/collections/shaped-types.md` rev 2.2 (§§ 3.4, 5, 8, 9, 11.1–11.2, 12, 13, 14.2, 18, 24.4, 27, 29.3, 29.5), `rules/memory/storage.md` § 16.4, `rules/memory/allocation.md` § 8(6), `rules/compiler/compiler_known_members.md` (shaped A/B classification, static ToShape, registry facts, ShapeError union), `rules/library/core-library.md` § 24. Implemented: core `StorageError` enum, `ShapeError` error union with `InitError(StorageError)` and `InvalidStep` removed (no other normative use found), core `StorageRequest` struct (`sec/core/error.sec`, `sec/core/storage.sec`; `TestShapedClosureErrorAndRequestDeclarations`). Known hole (verified 2026-10-21): `ShapedStorageRequest[Rank]`, `MemorySpace.Ordinary`, the shaped families' `init` overloads, static `ToShape`, the storage producers and algebra are not implemented in Sema/IR. MD-016 stays open for the non-shaped A/B audit of § 18; its shaped portion is recorded as applied.
- **Decision:** MD-016 (shaped/public storage portion)
- **Created:** 2026-10-21
- **Updated:** 2026-10-21
- **Revision:** 1.0
- **Sec version:** 0.1
- **Canonical path:** `rules/corrections/applied/md016-shaped-public-surface-closure-20261021.md`
- **Prior applied correction:** `rules/corrections/applied/md016-shaped-metadata-declarations-correction-20261018.md`
- **Primary owners:** `rules/collections/shaped-types.md`, `rules/memory/storage.md`, `rules/compiler/compiler_known_members.md`, `rules/library/core-library.md`
- **Other authorities:** `rules/declarations/impl.md`, `rules/declarations/unions.md`, `rules/types/default_values.md`, `rules/memory/allocation.md`, `rules/memory/arena.md`, `rules/memory/borrowing.md`, `rules/memory/copy_move.md`, `rules/memory/destruction.md`, `rules/memory/layout.md`
- **Replaces:** The older shaped `Create(...)`, instance-consuming `ToShape`, and incompatible error/contiguity passages listed below. Does **not** replace MD-016 part 1.

> **Integration status is distinct from design status.** All shaped design choices recorded here were accepted in the MD-016 discussion through 2026-10-21. The public web source for `shaped-types.md` was older than the recorded applied part 1, and a fresh `main` SHA could not be independently verified in this session. Integrators MUST compare the current checkout to the part-1 correction and the rulebooks before applying changes. Do not overwrite already-integrated part-1 declarations with an older web snapshot.

---

## § 1. Purpose, authority, and MD-016 scope

**§ 1(1).** This correction is the complete consolidated shaped-types decision record for MD-016, not a sequence of partial deliverables. Its numbered paragraphs are stable citation targets. Subsequent revisions SHALL append subordinate numbered paragraphs or explicitly supersede them; renumbering is prohibited.

**§ 1(2).** The normative A/B rule is `compiler-known-fundamentals-cross-rulebook-correction-20260907.md` § 21: **A** = a real source-visible Sec declaration, with a canonical owner and complete declaration; **B** = a compiler-semantic construct, explicitly *not* a Sec declaration, with a complete structural contract. A compiler-known identity may need special Sema handling even when its user-visible declaration is A.

**§ 1(3).** This document finalizes the shaped family and its storage-facing declaration decisions. The five metadata types and original `ShapeError` were handled by part 1; the replacements and extensions here take precedence where inconsistent. Do not introduce separate shadow declarations or LSP-only signatures.

**§ 1(4).** The repository-level `missing-decisions.yaml` definition of MD-016 is broader than shaped types: it also covers other incomplete compiler-known declarations. Its *entire* umbrella entry MUST NOT be marked resolved solely because this shaped correction has been applied. Repository-wide closure requires the independent A/B audit of the non-shaped entries as specified in § 18. This is a verification gate, not a new language-design question.

## § 2. Canonical A/B inventory for shaped types

**§ 2(1).** The following are public nominal Sec type identities with source-visible declarations (**A**, compiler-provided and available without an optional stdlib import): `Shape[Rank]`, `Strides[Rank]`, `TensorLayout[Rank]`, `Axes[Rank]`, `AxisList[Count]`, `StorageRequest`, `ShapedStorageRequest[Rank]`, `StorageError`, and `ShapeError`. Their owning declarations are synchronized into `shaped-types.md`, with general `StorageRequest` semantics also owned by `storage.md`. The compiler registry names *one symbol* for each identity. `MemorySpace` is also a **public, source-usable type identity**, but its intentionally opaque compiler-provided declaration is classified **B** for the formal § 21 declaration audit: no ordinary Sec source declaration exposes its representation. Being public and being category A are not synonyms.

**§ 2(2).** `vector[T,N]`, `matrix[T,Rows,Columns]`, static `tensor[T,D0,D1,...]`, runtime `tensor[T,Shape[Rank]]`, and `tensor_view[T,Rank]` are **B with respect to their representation/grammar-generating family**, i.e. compiler-known type forms and intrinsic operations, not ordinary user-defined `type ... struct` declarations. They nevertheless appear as real source-level *types*. Their exhaustive programmer-visible form, members, and invariants are specified in §§ 6–13; a compiler or LSP may synthesize readable, non-editable signatures but MUST NOT pretend that a user-writable nominal struct definition is their implementation.

**§ 2(3).** The following are **B (not public Sec types)**: `MemorySpaceKind`, `StorageOrigin`, `InvalidationDomain`, `ValidityEpoch`, backing identity, mutable-access/overlap proof, logical-to-physical index mapping, physical offset, and internal layout/provenance metadata. Their meanings stay with `storage.md` and memory rulebooks, not public structs or enums.

**§ 2(4).** `MemorySpace` is *public* but has intentionally hidden representation and compiler-defined construction; the § 21 **B** structural contract is § 4. Do not invent a user-constructible `type MemorySpace uint`, a public writable field, a numeric cast, or a made-up `opaque` keyword. The compiler-provided public identity is synthetic rather than an ordinary compilable Sec source declaration. Generated definitions must clearly label representation as compiler-private rather than presenting non-Sec syntax as compilable code.

## § 3. Existing metadata types — retain part 1 exactly

**§ 3(1).** The accepted part-1 declarations, not new alternatives, own the complete surface:

| Type | Public members | Construction | Indexing |
|---|---|---|---|
| `Shape[Rank]` | read-only `Rank: uint` | `init(extents: uint[Rank])`, infallible; literal `Shape[Rank] { ... }` | read-only extent `uint` |
| `Strides[Rank]` | read-only `Rank: uint` | `init(strides: int[Rank])`, infallible; literal `Strides[Rank] { ... }` | read-only stride `int` |
| `TensorLayout[Rank]` | exactly `DenseRowMajor`, `DenseColumnMajor`, `Strided` | enum variants only | none |
| `Axes[Rank]` | read-only `Rank: uint` | `init(axes: uint[Rank]) ShapeError`; valid literal | read-only axis `uint` |
| `AxisList[Count]` | read-only `Count: uint` | `init(axes: uint[Count]) ShapeError`; valid literal | read-only axis `uint` |

**§ 3(2).** `Rank`/`Count` are compile-time nonnegative integer parameters of these compiler-known forms. Metadata types do NOT inherit the shaped-payload `Len`, `SizeOf`, `Ptr`, `Layout`, `MemorySpace`, `ToString`, mutable indexed operations, or generic iteration merely because their representation might be array-like.

**§ 3(3).** `Shape[0]` and `AxisList[0]` are valid metadata. Rank-zero *tensor values* are forbidden by § 6(4). Part-1 constructor spelling and member documentation are reused verbatim; no duplicate public declaration for the same metadata identity may be created.

## § 4. Public `MemorySpace` identity

**§ 4(1).** `MemorySpace` is a nominal, opaque, compiler-known *value* identity for one memory domain; it is not a category enumeration. Equivalence is identity equality. Arbitrary integer casts, user-defined arbitrary values, and reinterpretation of `MemorySpaceKind` as a public enum are forbidden.

**§ 4(2).** The canonical universally available named value is `MemorySpace.Ordinary`. A target/platform definition may expose additional distinctly named `MemorySpace` constants. Such declarations are target-qualified public identities; their complete enumeration is owned by the *selected target definition*, not by a fabricated target-independent master list. Absent target identities are not synthesizable.

**§ 4(3).** For each target-defined identity the platform contract SHALL specify at least: stable identity, availability, legal accesses and execution entities, alignment/width, addressability, coherence/atomicity/volatility, supported storage origins, transfer capabilities, and applicable diagnostics. Distinct GPU devices or MMIO spaces remain distinct values even if both share `MemorySpaceKind.TargetDefined` or `.MMIO` internally.

**§ 4(4).** Every relevant shaped owner and view exposes read-only `MemorySpace: MemorySpace`. Views inherit it from backing storage. A logical SSA value may carry this semantic placement without having materialized backing; observing it does not allocate or transfer memory. It may be changed only through an explicit storage-producing operation.

## § 5. Exact public storage request and storage error declarations

**§ 5(1).** The public storage request declarations are these nominal structs, with **only** these fields. The following is the canonical source-visible form to synchronize into the owning rulebooks (the struct spelling must match Sec's canonical `type Name struct` syntax):

```sec
type StorageRequest struct {
    /** Requested destination memory identity; None imposes no extra restriction. */
    MemorySpace: Option[MemorySpace]

    /** Minimum byte alignment; None uses natural required alignment. */
    MinAlignment: Option[uint]
}

type ShapedStorageRequest[Rank] struct {
    /** General memory-domain and alignment requirements. */
    Storage: StorageRequest

    /** Required destination layout; None selects the operation's canonical layout. */
    Layout: Option[TensorLayout[Rank]]
}
```

**§ 5(2).** `None` does not demand ordinary memory. `Some(MemorySpace.Ordinary)` explicitly requires ordinary memory. `MinAlignment: None` requires at least natural alignment; `Some(n)` requires a valid minimum `n` bytes. An unsatisfied explicit request is an error, never an advisory hint. Allocator and `Arena` ownership are *not fields* of either request.

**§ 5(3).** `ShapedStorageRequest.Layout` is restricted to `DenseRowMajor` or `DenseColumnMajor` for *new owning storage* in Sec 0.1. The value `Strided` remains a valid descriptor of existing backing and views, but `Some(Strided)` cannot select an otherwise unspecified set of strides for new owning storage; that request fails with `StorageError.UnsupportedLayout` when runtime checked (or is statically rejected when proven invalid).

**§ 5(4).** The canonical shared storage error domain has six variants and must not be duplicated as `ShapedStorageError`:

```sec
enum StorageError error {
    // Required storage could not be acquired.
    AllocationFailed

    // The selected target/provider cannot satisfy the requested memory identity.
    UnsupportedMemorySpace

    // The requested destination layout cannot be produced.
    UnsupportedLayout

    // The requested alignment cannot be met.
    AlignmentUnsatisfied

    // An explicit storage transfer failed.
    TransferFailed

    // Required size or address arithmetic overflowed its supported domain.
    SizeOverflow
}
```

**§ 5(5).** An underlying `AllocationError` may be translated to `StorageError.AllocationFailed` at the shaped-storage boundary; this mapping shall preserve diagnostics/cause details internally where useful. No speculative allocator failure may be inserted into otherwise pure shaped arithmetic.

## § 6. Shaped family structural contracts and rank limits

**§ 6(1).** Owning source type families (special compiler-recognized forms) are:

```sec
vector[T, N]
matrix[T, Rows, Columns]
tensor[T, D0, D1, ...]
tensor[T, Shape[Rank]]
```

Their complete common instance property surface is enumerated in § 7, their initialization in § 9, and their intrinsic method surfaces in §§ 10–12. The static variadic `tensor` form is special to the compiler; it does not introduce ordinary variadic generic parameters.

**§ 6(2).** The non-owning family is `tensor_view[T, Rank]`, valid as `ref tensor_view[T, Rank]` or `ref mut tensor_view[T, Rank]` carrying backing provenance, effective borrow authority, invalidation domain, and memory-space constraints. It cannot independently own elements or their storage. The descriptor may be held as SSA metadata; the `ref` spelling does not require separately allocated descriptor storage.

**§ 6(3).** Extents may be zero, and total `Len` is the checked product of extents; it is never only the first extent. `vector[T,0]` and `matrix[T,0,M]` are valid empty shaped values.

**§ 6(4).** In Sec 0.1 `tensor[T, Shape[0]]` and `tensor_view[T,0]` are INVALID *value type forms*. An operation whose result rank is zero yields scalar `T`, not a tensor. `Shape[0]` and `AxisList[0]` remain valid metadata. Rank ≥ 1 is required for shaped values.

**§ 6(5).** `vector` and `matrix` are not dynamically growable collections. Owning shaped results may stay in logical form, SSA, registers, or an equivalent legal representation, unless an observable operation requires actual addressable backing. No hidden fallible allocation is allowed to rescue an invalid lifetime or borrow.

## § 7. Exhaustive common shaped properties

**§ 7(1).** For each applicable instance of the four owning shaped families and `ref`/`ref mut tensor_view`, the public member surface contains the following **read-only properties** (not methods):

```sec
property Rank: uint { get }
property Shape: Shape[Rank] { get }
property Len: uint { get }
property Strides: Strides[Rank] { get }
property Layout: TensorLayout[Rank] { get }
property MemorySpace: MemorySpace { get }
property IsContiguous: bool { get }
property SizeOf: uint { get }
unsafe property Ptr: RawPtr[T] { get }
```

Here `Rank` in `Shape[Rank]`, `Strides[Rank]`, `TensorLayout[Rank]` is the resolved compile-time rank of that receiver; the template above is the *single exact member list*, instantiated by the compiler for each family. `Ptr` exists only on addressable instances and requires unsafe access. It must not force materialization of a pure temporary.

**§ 7(2).** Type-associated read-only `Rank`, `Shape`, and `Len` are available only where the entire value is a compile-time fact. The rank is associated on `tensor_view[T,Rank]` and runtime-shaped `tensor[T,Shape[Rank]]`, but their instance-specific extents and element counts are not type-associated values. `Type.SizeOf` continues to denote physical complete-type layout, unlike instance `SizeOf`.

**§ 7(3).** `SizeOf = Len × element payload size` checked in the applicable numeric/target representation. It is not descriptor size, allocation size, occupied address span, or distinct-byte count for a broadcast. In particular, `Ptr + SizeOf` is a usable linear buffer only if its layout, contiguity, addressability, and FFI access conditions all hold.

**§ 7(4).** `IsContiguous` describes one physically unbroken, non-overlapping representation of the logical elements, not necessarily row-major *logical traversal*. Canonical dense row-major, dense column-major, and an ordinary transpose of a dense matrix may all return `true`. A `Strided` layout may still be physically contiguous. A reverse view with negative traversal stride returns `false` under the agreed `.Ptr` origin convention; actual overlap from broadcasting returns `false`.

**§ 7(5).** When `Len == 0`, `IsContiguous == true` vacuously, regardless of metadata layout. No element storage need exist; `Ptr` must not be dereferenced. Pointer representation for this empty case is target-dependent but never a license for unsafe access.

**§ 7(6).** `Ptr` is a raw pointer to the view's logical origin, not a guarantee that subsequent logical elements are consecutive. A contiguous bulk buffer additionally requires the origin to be the start of the contiguous physical range and a compatible expected element order. Contiguity does not imply row-major layout.

## § 8. Constructor selector and `init` contract

**§ 8(1).** `new Type(args...)` selects an `init(args...)` overload or a permitted implicit zero-argument construction path. `new` is not an allocator and `init` is not a separate factory named `Create`. Source-visible `init(...)` declarations have no `fn`, no explicit success return type, and, when fallible, **only the error type** after the parameters. The completed `new` expression has the owning shaped type, not `Result` as its success type.

**§ 8(2).** There are two *algorithms* selected through `init` overloads, not two competing constructors: (i) initialize every element with its defined default; (ii) consume an existing `T[]` owning element collection in logical order. Both may additionally select a storage request and/or explicitly borrowed allocation authority.

**§ 8(3).** Static shaped types derive their complete shape from type arguments; a runtime-shaped tensor **must** receive `shape: Shape[Rank]` explicitly on construction. The compiler MUST NOT invent an all-zero shape or leave extents unspecified.

**§ 8(4).** Default-element construction is valid only if `T` has a valid semantic default for each required element. In particular, defaulting `int` yields zero, not merely zeroed memory; a `File` is not fabricated from zero bytes. A type lacking a default may still be used with a statically proven zero-element shape. Runtime possibility of zero elements does not alone prove that a default-free general constructor is valid.

**§ 8(5).** The owning-element constructor checks exact `elements.Len == shape.Len`; static proofs are performed at compile time where possible, otherwise checked at runtime. It supports move-only `T`. Element transfer follows normal ownership rules; no implicit deep copy is inserted. The consuming parameter is `-> elements: T[]`, and a named argument is passed with `<-elements`.

**§ 8(6).** By ordinary call semantics, an owner is consumed at call entry. If an initializer discovers an element-count mismatch or storage failure afterward, the caller's source binding does not automatically revive. The initializer must clean up the consumed owner/partially initialized elements. No special transactional rollback is added.

**§ 8(7).** An explicit Arena is passed by mutable borrowing; it is not consumed. The exact Arena allocation, lifetime, reset, and release guarantees remain owned by `arena.md`. Shaped initialization may reuse source backing only when representation, physical layout, alignment, memory space, ownership, and lifetime permits it. It MUST NOT *promise* reuse for an arbitrary `T[]`.

**§ 8(8).** A valid infallible implicit `init()`-equivalent path may exist for a statically shaped defaultable type if required storage/initialization is provably infallible. A fallible storage acquisition must instead select a fallible declared `init` and use `try new`. There is no automatic `init()` that invents allocation success.

## § 9. Canonical shaped `init` overload families

**§ 9(1).** The following signatures are the compiler-known, source-visible constructor surface. A family instance is instantiated at concrete compile-time rank. The complete public overload set is composed of each base form and its equivalent explicit-Arena form as shown here; no per-dimension ad hoc constructors exist. The `init` bodies are compiler-provided; signature declarations can be emitted in read-only generated definitions for tooling.

**§ 9(2).** For a **static vector**:

```sec
impl vector[T, N] {
    // Default-initialize N elements under an explicit storage request.
    init(request: ShapedStorageRequest[1]) ShapeError

    // Consume N existing elements.
    init(-> elements: T[]) ShapeError

    // Consume existing elements with a required storage placement.
    init(-> elements: T[], request: ShapedStorageRequest[1]) ShapeError

    // Explicit Arena variants; Arena is borrowed, not consumed.
    init(ref mut arena: Arena, request: ShapedStorageRequest[1]) ShapeError
    init(ref mut arena: Arena, -> elements: T[]) ShapeError
    init(ref mut arena: Arena, -> elements: T[], request: ShapedStorageRequest[1]) ShapeError
}
```

**§ 9(3).** For a **static matrix**:

```sec
impl matrix[T, Rows, Columns] {
    init(request: ShapedStorageRequest[2]) ShapeError
    init(-> elements: T[]) ShapeError
    init(-> elements: T[], request: ShapedStorageRequest[2]) ShapeError

    init(ref mut arena: Arena, request: ShapedStorageRequest[2]) ShapeError
    init(ref mut arena: Arena, -> elements: T[]) ShapeError
    init(ref mut arena: Arena, -> elements: T[], request: ShapedStorageRequest[2]) ShapeError
}
```

**§ 9(4).** For a **static tensor** with compiler-special variadic extent pack and statically known rank `Rank = count(Dims)`:

```text
// Structural template for the compiler-known variadic form, NOT a user
// declaration of a general variadic generic type:
impl tensor[T, Dims...] {
    init(request: ShapedStorageRequest[Rank]) ShapeError
    init(-> elements: T[]) ShapeError
    init(-> elements: T[], request: ShapedStorageRequest[Rank]) ShapeError
    init(ref mut arena: Arena, request: ShapedStorageRequest[Rank]) ShapeError
    init(ref mut arena: Arena, -> elements: T[]) ShapeError
    init(ref mut arena: Arena, -> elements: T[], request: ShapedStorageRequest[Rank]) ShapeError
}
```

**§ 9(5).** For a **runtime-shaped tensor**:

```sec
impl tensor[T, Shape[Rank]] {
    // Shape is mandatory and element defaults are required for Len > 0.
    init(shape: Shape[Rank]) ShapeError
    init(shape: Shape[Rank], request: ShapedStorageRequest[Rank]) ShapeError

    // Consume an existing owning collection, preserving logical element order.
    init(shape: Shape[Rank], -> elements: T[]) ShapeError
    init(shape: Shape[Rank], -> elements: T[], request: ShapedStorageRequest[Rank]) ShapeError

    // Explicitly selected mutable Arena (borrowed).
    init(shape: Shape[Rank], ref mut arena: Arena) ShapeError
    init(shape: Shape[Rank], ref mut arena: Arena, request: ShapedStorageRequest[Rank]) ShapeError
    init(shape: Shape[Rank], ref mut arena: Arena, -> elements: T[]) ShapeError
    init(shape: Shape[Rank], ref mut arena: Arena, -> elements: T[], request: ShapedStorageRequest[Rank]) ShapeError
}
```

**§ 9(6).** Examples of the *single* lifecycle-construction system:

```sec
let shape := Shape[2] { 2, 3 }
let owner := try new tensor[int, Shape[2]](shape)

let files := GetFiles()
let moved := try new tensor[File, Shape[2]](shape, <-files)

let placed := try new tensor[int, Shape[2]](shape, ref mut arena, request)
```

**§ 9(7).** A constructor with only storage failure may use `StorageError` directly only if it is a distinct signature whose contract cannot produce a shape/element-count error; for the uniform shaped constructor family above, `ShapeError` is the canonical error channel. Every failure path must be reflected in the declared error channel. The parameterless construction path is not duplicated as a redundant `init()` declaration.

**§ 9(8).** `tensor_view` has no owning `init`. A view is obtained by borrowing and affine mapping, never by constructing an unverified independent view/owner. No public `New` or `Create` constructor is supplied for it.

## § 10. Public `ShapeError` extension, no `ShapedInitError`

**§ 10(1).** There is exactly ONE canonical `ShapeError` nominal type. Convert the nine-case enum from part 1 to the following **closed error union** (the formerly slicing-specific `InvalidStep` is removed from `ShapeError` after the reference audit required by § 10(3)):

```sec
type ShapeError union error {
    // The supplied rank does not match the required rank.
    RankMismatch

    // Two extents or the number of supplied elements are incompatible.
    DimensionMismatch

    // An element-count product overflows the allowed count domain.
    ElementCountOverflow

    // A no-copy reshape cannot preserve the logical value mapping.
    InvalidReshape

    // Explicit broadcasting cannot be represented for these shapes.
    InvalidBroadcast

    // Contracted axes or their extents are invalid or incompatible.
    InvalidContraction

    // A referenced axis is outside the legal source rank.
    InvalidAxis

    // An axis that must be unique appears more than once.
    DuplicateAxis

    // Storage or placement failed during shaped initialization.
    InitError(StorageError)
}
```

**§ 10(2).** `ShapedInitError` and `ShapedStorageError` must NOT be introduced. `StorageError` remains the direct return type of `Materialize`, `TransferTo`, `Relayout`, and other explicit *storage-producing* operations as defined in § 11. `ShapeError` is the canonical runtime shape/constructor error and transports init storage failure as `ShapeError.InitError(StorageError.<case>)`. Construction code must explicitly create the named union variant; this is not a new implicit conversion rule. Callers handle one shaped error domain.

**§ 10(3).** Shaped slicing, ranges, and zero/invalid runtime `step` use general `RangeError`, and scalar indexing uses `IndexError`. Constant `step 0` is a compile-time error. Before removing `ShapeError.InvalidStep`, search the *current checkout* for any non-slicing normative use and migrate or explicitly document it; do not silently delete genuinely distinct behavior. Part-1 nine-variant enum references, compiler-known metadata registry descriptions, exhaustive matches, diagnostics, and tests must all update together.

**§ 10(4).** A public enum cannot carry `InitError(StorageError)` payload. This is a deliberate enum-to-union evolution of the same nominal identity, not two conflicting declarations. Generated API docs/LSP must list the full nine-case union above.

## § 11. Exhaustive storage-producing operation contracts

**§ 11(1).** `Materialize()` and `Materialize(request: ShapedStorageRequest[Rank])` are non-consuming operations on a borrowed shaped view that return a NEW owning `tensor[T, Shape[Rank]]` with `StorageError` failures. They may allocate, pack strided/reversed/broadcast logical elements, and must preserve logical element order. The original view and backing are unchanged. Generic availability requires valid *duplicating/copying* semantics for `T`; move-only payload may not be bit-copied or silently consumed. Invalid duplication capability is a compile-time diagnostic.

**§ 11(2).** `TransferTo(request: ShapedStorageRequest[Rank])` on an owning shaped value produces a NEW owner of the same shaped family/shape, leaving the source owner alive; it may copy, map or use a platform transfer primitive and may fail with `StorageError`. Success of the synchronous method means transfer and destination initialization have completed. Asynchronous transfers need a distinct API/handle and lifetime contract.

**§ 11(3).** `Relayout(layout: TensorLayout[Rank])` on an owning shaped value produces a NEW owner with equivalent logical elements and required layout; the original owner survives. It may allocate and reorder elements and fails with `StorageError`. Explicit `Strided` as new owning output is rejected per § 5(3). A backend may fuse compatible transfer/relayout steps without altering observable semantics.

**§ 11(4).** All non-consuming storage producers require a legal non-destructive duplication of `T`. Explicit fallible duplication is not silently invoked in ordinary copying. If a separately defined explicit operation uses fallible cloning, its contract must expose the effects/errors and is not silently this overload. No arbitrary move-only `File` can be duplicated without an independent legitimate resource-duplication operation.

**§ 11(5).** `Materialize`, `TransferTo`, and `Relayout` are normal named methods, **not** renamed to `init`. `init` is the lifecycle initialization invoked by `new`; these methods transform/produce results from existing values.

## § 12. Reshape, ToShape, affine views, and algebra

**§ 12(1).** `Reshape(newShape: Shape[NewRank])` is non-consuming and yields a borrowed `ref tensor_view[T,NewRank]`, or `ref mut` where authority and non-overlap prove it legal. It requires equal logical element count and representation of the *same logical element order* in the existing backing/strides. It never copies or allocates. Known-valid checks are infallible; runtime-dependent checks use `ShapeError.InvalidReshape`.

**§ 12(2).** `ToShape` is the compiler-known **static method associated with the owning source type**. Its first parameter is explicitly consuming `-> source: ...`, and the call site must pass `<-source` for a named binding; it does not consume implicit instance `self`. Example for a runtime-shaped source:

```sec
let flatShape := Shape[1] { image.Len }
let flat := try tensor[float32, Shape[3]].ToShape(<-image, flatShape)
```

`ToShape` yields a new owning shaped value, transfers the exact same backing ownership and `MemorySpace`, makes no allocation/copy, requires equal element counts and compatible logical mapping, and obeys normal consuming failure cleanup. A failed check after call entry does not automatically restore `image`.

**§ 12(3).** Physical contiguity alone is insufficient for no-copy reshape. For example, a transposed matrix can be physically contiguous yet flattening it in row-major logical order would require the sequence `[0,3,1,4,2,5]`, not a simple affine rank-one sequence. `Reshape` and `ToShape` reject that reordering; `Materialize` permits explicit copying.

**§ 12(4).** A `tensor_view` descriptor contains or derives rank, shape, signed element strides, origin, storage identity/provenance, and borrow authority. It does not independently own or extend backing lifetime. Descriptors can be represented as SSA and composed without heap allocation; `Transpose().Materialize()` may fuse the descriptors and pack the final output directly.

**§ 12(5).** `Permute(axes: Axes[Rank])` and rank-two `Transpose()` are non-copying, non-allocating borrowed view transformations preserving backing and memory space. They permute axes, strides, and recompute layout/contiguity. Slicing uses exactly one selector per input axis; scalar selectors remove axes, range selectors retain axes. All-scalar selection is an assignable `Place[T]`, not rank zero.

**§ 12(6).** `step < 0` yields reverse traversal with negative stride; `step == 0` is invalid. Negative strides alone do not prohibit `ref mut` when every logical index maps to a distinct backing element. Broadcast with actual zero-stride aliasing cannot expose overlapping mutable element references; `BroadcastTo(targetShape)` is explicit, right-aligned, non-consuming, and does not allocate.

**§ 12(7).** No hidden broadcasting or transfer between distinct incompatible memory spaces is permitted in arithmetic. `*` is elementwise, contextual infix `x` represents defined algebraic multiplication, and `Contract` requires explicit paired axis lists. Rank-zero contraction results are scalar. Ordinary scalar type rules and Sec's unit algebra govern result types.

**§ 12(8).** An algebraic reduction over zero elements requires the result type's additive identity and returns that identity. A matrix product with zero inner extent yields all additive identities; if the result type lacks such an identity and empty reduction is possible, compilation rejects the operation. Static and runtime zero extents have the same semantics.

**§ 12(9).** `Magnitude` requires valid product, sum, and square-root scalar semantics and retains the resulting physical dimension. `Normalize` divides by magnitude, returns an `Option[vector[...]]` of the properly inferred scalar/unit type, and yields `None` on zero magnitude. It never injects implicit integer-to-float, decimal rounding, or a new unit-conversion rule. Empty vectors follow the same zero/`None` rule when scalar operations are defined.

## § 13. Destruction, partial initialization, lifetime, and access

**§ 13(1).** Owning shaped values destroy initialized owned elements exactly once in **reverse logical index order**, independent of row-major/column-major physical placement; a `[2,3]` matrix destroys `[1,2]`, `[1,1]`, `[1,0]`, `[0,2]`, `[0,1]`, `[0,0]`. Trivial destruction can be optimized away but nontrivial observable order is fixed.

**§ 13(2).** Partially initialized shaped values clean up only successfully initialized elements, in reverse *successful initialization order*, plus acquired backing resources. The incomplete outer value never becomes readable; custom completed-value `free()` must not be called on a never-completed outer value.

**§ 13(3).** Empty shaped values destroy no elements. `tensor_view` never destroys backing elements or releases backing ownership; any descriptor cleanup must not affect those objects.

**§ 13(4).** A returned `ref tensor_view[...]`/`ref mut tensor_view[...]` is valid only when its lifetime/provenance can be tied to a valid external backing owner. Escaping a local owner's lifetime is forbidden; an optimizer must not silently materialize/copy to repair it. Active overlapping borrows prohibit consuming or invalidating the source backing.

**§ 13(5).** `for` traverses shaped elements in logical index order regardless of layout; by-value iteration requires legal copy semantics, `for ref` borrows, and `for ref mut` requires exclusive, non-overlapping access. No hidden per-element allocation is introduced.

## § 14. Compiler-known method/member structural registry

**§ 14(1).** The following is the **complete named intrinsic shaped operation inventory** for Sec 0.1; no additional public aliases are created by MD-016:

| Operation | Receiver/case | Result identity / effect |
|---|---|---|
| `Reshape` | shaped owner/view, borrow | view of `NewRank`, same backing; `ShapeError` when runtime checked |
| `ToShape` | static on each owning family; consumes source | new owning tensor, same backing; `ShapeError` when checked |
| `Materialize` | `ref tensor_view` | new runtime-shaped owner; `StorageError`, may allocate |
| `TransferTo` | owning shaped instance | new independent owner; `StorageError`, may allocate/transfer |
| `Relayout` | owning shaped instance | new independent owner; `StorageError`, may allocate/reorder |
| `Permute` | shaped owner/view | borrowed permuted view; no allocation |
| `Transpose` | rank two shaped owner/view | borrowed transposed view; no allocation |
| `BroadcastTo` | shaped owner/view | shared or legally mutable borrowed view; checked `ShapeError` |
| `Dot` | compatible vectors | scalar, additive reduction |
| `Outer` | compatible vectors | owning logical `matrix` |
| `Magnitude` | compatible vector | scalar derived by multiply/sum/sqrt |
| `Normalize` | compatible vector | `Option[vector[...]]` |
| `Cross` | extent-3 vectors | logical extent-3 vector |
| `Contract` | tensors + two `AxisList[Count]` | scalar or logical tensor; checked `ShapeError` |
| `x` | defined matrix×matrix or matrix×vector | shaped algebraic product; checked shape if runtime needed |

**§ 14(2).** The compiler-known registry SHALL record stable IDs, receiver patterns, rank/type substitutions, argument names and ownership modes, result/rank conditions, error channels, effects (allocation/transfer/unsafe), addressability conditions, borrow provenance, and target availability. The table is *not* a license to generate fake concrete Sec signatures for an unconstrained variadic/rank-changing builtin. For each concrete receiver, LSP must synthesize an exact fully specialized signature and documentation from those structural facts.

**§ 14(3).** In addition to operations, the *complete* public property set is exactly § 7; metadata-only types are exactly § 3; storage contract/error types are §§ 4–5 and § 10. The compiler registry, core authority, Sema, formatter, diagnostics, LSP, and code generation must agree on this identity map.

**§ 14(4).** For runtime-shaped owners, the following are exact generic *source-facing signature projections* of the compiler-known operations. These are generated declarations, not permission for an ordinary user module to implement compiler-owned builtins. The methods are omitted from a concrete receiver when their scalar, shape, copying, or target constraints are not satisfied:

```sec
impl tensor[T, Shape[Rank]] {
    /** Produces a borrowed affine view without moving or copying storage. */
    fn Reshape[NewRank](newShape: Shape[NewRank]) Result[ref tensor_view[T, NewRank], ShapeError]

    /** Moves the complete source owner into a new runtime shape. */
    static fn ToShape[NewRank](-> source: tensor[T, Shape[Rank]], newShape: Shape[NewRank]) Result[tensor[T, Shape[NewRank]], ShapeError]

    /** Produces an independent owner satisfying destination storage requirements. */
    fn TransferTo(request: ShapedStorageRequest[Rank]) Result[tensor[T, Shape[Rank]], StorageError]

    /** Produces an independent owning layout while preserving logical elements. */
    fn Relayout(layout: TensorLayout[Rank]) Result[tensor[T, Shape[Rank]], StorageError]

    /** Borrows a view whose dimensions and strides are permuted. */
    fn Permute(axes: Axes[Rank]) ref tensor_view[T, Rank]

    /** Borrows an explicit right-aligned broadcast view. */
    fn BroadcastTo[NewRank](targetShape: Shape[NewRank]) Result[ref tensor_view[T, NewRank], ShapeError]
}
```

**§ 14(5).** Mutable view overloads are *separate constrained resolutions* of the same names, preserving `ref mut tensor_view` only where the backing reference is exclusive, every logical result index is non-overlapping, and the transformation permits mutable authority. The compiler MUST NOT expose a `ref mut` overload for real broadcast expansion. On a borrowed `tensor_view`, `Materialize` has these two complete public return/error shapes:

```sec
// Compiler-owned generated member projections for ref tensor_view[T, Rank].
fn Materialize() Result[tensor[T, Shape[Rank]], StorageError]
fn Materialize(request: ShapedStorageRequest[Rank]) Result[tensor[T, Shape[Rank]], StorageError]
```

**§ 14(6).** Rank-two `Transpose()` is the parameterless permutation of `[1,0]`; when a statically shaped owner calls `TransferTo` or `Relayout`, the result preserves that original compile-time family and extents (rather than silently converting to a runtime-shaped tensor). The four source-owning families all expose an appropriately instantiated static `ToShape` consuming parameter. For a static source, `Rank` and the source extents are known type facts; `NewRank` is deduced from `newShape`.

**§ 14(7).** The remaining named algebraic operations have the following compiler-semantic complete shape/arity contracts: `Dot(other)` accepts two rank-one operands of equal length and returns the scalar additive reduction of pairwise products; `Outer(other)` accepts two rank-one operands and returns a rank-two logical matrix with their two extents; `Magnitude()` accepts one rank-one receiver and returns a scalar determined by sum-of-squares followed by square root; `Normalize()` accepts one rank-one receiver and returns `Option[vector[DerivedScalar, N]]`; `Cross(other)` accepts two extent-three rank-one operands and returns extent three; `Contract(other, leftAxes: AxisList[Count], rightAxes: AxisList[Count])` accepts two tensors, two equally long explicit axis lists, and returns the scalar or tensor of uncontracted axes in left-then-right order. `DerivedScalar` here describes the compiler's *resolved scalar algebra result*, not a new Sec type identifier. Operations with an unavailable element algebra or invalid static rank/shape are absent/rejected at compilation, never secretly overloaded with an arbitrary coercion.

## § 15. Diagnostic and compiler integration requirements

**§ 15(1).** Distinguish (i) static impossibility; (ii) a checked runtime `ShapeError`, `RangeError`, `IndexError` or `StorageError`; (iii) unsupported target capability; (iv) ownership/borrow conflict; (v) unsafe and addressability obligations. Never insert a hidden `try` for speculative allocation.

**§ 15(2).** Diagnoses must say what the programmer can do, e.g. wrong element count, `step 0`, borrowed storage cannot escape, `ToShape` needs `<-` and a source *type-qualified static* call, `Materialize` needs duplicable elements, or `Strided` cannot be an explicit new owning destination layout.

**§ 15(3).** Semantic IR and MLIR must distinguish logical shaped values, physical storage, views with their borrow provenance, successful element initialization, move/copy, actual materialization, physical transfer, and destruction. Lowering must not infer ownership from raw pointers or backend `memcpy` patterns.

**§ 15(4).** LSP hover/comments must be attached to actual declarations and members; comments precede their target declaration/variant. Do not expose hidden members as user-accessible fields, nor advertise `Create` after migration. All source examples and programmatic names in corrections are written in English.

## § 16. Required tests, grouped by boundary

**§ 16(1).** Compile-time acceptance and rejection: static vs runtime rank/shape; zero extents; forbidden rank-zero tensor/view; 64-bit/target-width checked shape product; error-union construction and exhaustive patterns; `init` overload choice; missing runtime shape; nondefaultable elements; static vs runtime element count; explicit `<-`; noncopyable storage production.

**§ 16(2).** Construction: implicit infallible default where permitted; fallible explicit `init`; arena selection and no transfer of arena ownership; request validation and exact layout; `StorageError` translated into `ShapeError.InitError`; cleanup of consumed `T[]` if count differs; partial initialization with failure injection; resource cleanup exactly once.

**§ 16(3).** Borrowing/view: zero-copy reshape, static `ToShape` qualification, consumption on success and on post-entry failure, active borrow rejection, lifetime escape rejection, mutable disjoint reverse view, mutable broadcast-overlap rejection, view composition/fusion.

**§ 16(4).** Storage: `MemorySpace.Ordinary`, two distinct platform identities of the same internal kind, unsupported requested space, alignment, `Some(Strided)` rejection on new owner, old strided view validity, row/column physical contiguity, transpose contiguity, reverse non-contiguity, broadcast overlap, empty `IsContiguous` and non-dereferenceable `Ptr`.

**§ 16(5).** Algebra: empty `Dot` and matrix product, no additive identity failure, unit-aware `Magnitude`, `Normalize` zero/empty/nonzero and unsupported scalar square root, rank-zero contraction scalar, explicit broadcast and incompatible memory space, reversed logical iteration and destruction order.

**§ 16(6).** Tooling and migration: stable member IDs; single owner declaration; all declared variants in LSP; part-1 enum-to-union update; generated `init` signature help; old instance-consuming `ToShape` rejected; `.Ptr` unsafe gating; `SizeOf` instance/type distinction; formatter preserves canonical spelling.

## § 17. Required integration sequence (one step and its test at a time)

**§ 17(1).** First identify `main` SHA and compare files with the current local checkout. Locate and retain the accepted part-1 `md016-shaped-metadata-declarations-correction-20261018.md`; do not create duplicate metadata types. Do not replace a newer source using the older cached version of `shaped-types.md`.

**§ 17(2).** Update `shaped-types.md` in the owning sections while preserving their existing section numbers: § 3 family/rank, §§ 4/6/7/15 part-1 declarations, §§ 8–11 contiguity/memory/storage, § 12 replace `Create` with `new`/`init`, § 13 storage producers, § 14 static `ToShape`, §§ 17–20 slicing/range/borrow, §§ 21–27 algebra/destruction, § 29 single `ShapeError` union + `StorageError`. Test each local change before continuing.

**§ 17(3).** Synchronize `storage.md` with § 4 opaque named identity and internal `MemorySpaceKind` split, and `allocation.md`/`arena.md` only as needed to replace illustrative `Create` forms for shaped targets. Do NOT globally rename legitimate `Create`/factory operations belonging to unrelated APIs.

**§ 17(4).** Synchronize `compiler_known_members.md` with *complete* new families/variants/overloads/required constraints, keeping existing stable IDs where possible. Update its part-1 description of the original nine-variant `ShapeError` to the replacement union in § 10. Reuse `core-library.md`'s compiler-owned shaped boundary; no user-extensible built-in impl is introduced.

**§ 17(5).** Implement/compiler-validate changed resolution, Sema, ownership, diagnostics, LSP, Semantic IR, and MLIR *one logical check at a time*. Do not claim implementation solely because semantics are documented.

**§ 17(6).** After *all* affected rulebooks and governance fragments are synchronized and tests pass, update `language-rulebook-status.md`, corresponding `governance/implementation-status-<book>.yaml` fragments by existing stable IDs (without replacing work already recorded), and the MD-016 entry as specified by § 1(4) and § 18. The legacy root implementation-status file is not authoritative.

## § 18. Non-shaped MD-016 audit boundary and closure condition

**§ 18(1).** The actual repository MD-016 descriptor explicitly includes incomplete compiler-known declarations **outside** shaped types. This correction settles the shaped decisions completely but does not silently assert completion of that repository-wide audit. The following preexisting groups must each be independently A/B-classified against their owning canonical rulebook and registry; a mere method-name list is not a complete declaration:

| Group | Canonical owner / audit target |
|---|---|
| `Ptr`, value/type `SizeOf`, `Len`, `IsEmpty`, global `len` | `compiler_known_members.md`, `core-library.md` |
| `ToString`, string/byte/char/rune conversions, string errors | `core-library.md`, `compiler_known_members.md` |
| Numeric `Min`, `Max`, `Bits`, `Epsilon`, `Infinity`, `NegativeInfinity`, `NaN`, `Scale` | `compiler_known_members.md`, numeric core definitions |
| Collection `Fill`, `Reverse`, `Append`, `Insert`, `Remove`, `Contains`, capacity `Reserve`/`WithCapacity`, etc. | `collections.md`, `compiler_known_members.md` |
| `RawPtr[T]` read/write/volatile/arithmetic | `raw_pointers.md`, `core-library.md`, `compiler_known_members.md` |
| `Arena`/allocation known members | `arena.md`, `allocation.md`, `compiler_known_members.md` |
| `Iterator[T]`/`Next`, `for` lowerable protocol | `flowcontrol_for.md`, `compiler_known_members.md` |

**§ 18(2).** For every *A* non-shaped item: identify the owning module and provide its complete canonical source declaration including exact receiver, parameters, return/error type, availability, and comments. For every *B* item: say explicitly that it is NOT a Sec declaration and document its entire structural contract (receiver eligibility, shape/arguments, result, effects, access/ownership, target availability, and diagnostics). Keep existing declarations already completed by other applied corrections rather than introducing duplicates.

**§ 18(3).** For closure of the umbrella `MD-016`, the repository must confirm no public compiler-known type/member remains only a prose mention or an incomplete pseudo-signature after the audit. This requires a validation against the actual live tree, not a claim based on inaccessible/cached files. If non-shaped A/B records remain incomplete, keep `MD-016` open but mark the *shaped portion* accepted/applied in its description. This preserves correct governance and avoids blocking unrelated work on falsely claimed closure.

## § 19. Accepted decision ledger (historical traceability)

**§ 19(1).** The present document consolidates the following accepted decisions: part-1 metadata; static type-qualified consuming `ToShape` using `<-`; no rollback; public opaque per-platform `MemorySpace` identities; request structs; owning vs borrowed view; materialization, transfer and relayout; shared `StorageError`; `RangeError` for slicing `step`; additive identity for empty reductions; scalar/unit-derived `Magnitude`/`Normalize`; reverse-logical destruction order; `Strided` description but not a new owning request; physical `IsContiguous` definition (accepted provisionally); empty contiguous views; scalar rank-zero output, no rank-zero tensor/view; `new` selecting overloaded `init`; explicit runtime shape; default vs consuming collection construction; arena separate; union `ShapeError` with `InitError(StorageError)` replacing extra shaped constructor error types.

**§ 19(2).** The contiguity decision was accepted as **provisional** (the user said it was acceptable *for now*). It is binding for this correction unless superseded, but the integration notes and tests must preserve that status so a future revision can revisit it explicitly without rewriting historical decision records.

**§ 19(3).** Unrelated language semantics and compiler-known entries are not changed implicitly by this correction; in particular no new general implicit move, fallible-copy, source-level variadic generics, implicit widening, or source-visible `MemorySpaceKind` is introduced.

---

## Authoritative references (repository paths)

1. `missing-decisions.yaml`, MD-016.
2. `rules/corrections/applied/md016-shaped-metadata-declarations-correction-20261018.md` (already applied part 1; integration checkout must be used).
3. `rules/corrections/applied/compiler-known-fundamentals-cross-rulebook-correction-20260907.md`, especially § 21.
4. `rules/collections/shaped-types.md`, §§ 1–34.
5. `rules/compiler/compiler_known_members.md`, especially “Shaped types”, “Shaped metadata families (MD-016 part 1)”, and the registry contract.
6. `rules/library/core-library.md`, § 24 shaped compiler/core boundary.
7. `rules/memory/storage.md`, §§ 16–18.
8. `rules/memory/allocation.md`, §§ 2–10; `rules/memory/arena.md`.
9. `rules/declarations/impl.md`, §§ 15–18; `rules/declarations/unions.md` §§ 2–3.
10. `rules/types/default_values.md`; `rules/memory/borrowing.md`; `rules/memory/copy_move.md`; `rules/memory/destruction.md`.
