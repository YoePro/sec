# Shaped Types

**Status:** Canonical normative rulebook
**Language version:** Sec 0.1
**Document revision:** 2.0
**Created:** 2026-08-13
**Last updated:** 2026-08-13
**Replaces:** the previous `rules/collections/shaped-types.md` revision 1.
**Collection semantics:** `rules/collections/collections.md`
**Storage semantics:** `rules/memory/storage.md`, `rules/memory/memory_model.md`
**Compiler-known members:** `rules/compiler/compiler_known_members.md`
**MLIR and lowering:** `rules/mlir/`

---

# 1. Purpose

This rulebook defines Sec's shaped-value model.

It owns the source-language semantics of:

```text
vector
matrix
tensor
tensor_view
Shape
Strides
TensorLayout
Axes
AxisList
ShapedStorageRequest
```

It also defines how shaped values interact with:

```text
MemorySpace
StorageRequest
ownership
borrowing
indexing
slicing
layout
contiguity
materialization
storage transfer
operators
vector algebra
matrix multiplication
tensor contraction
```

Ordinary collections such as fixed arrays, dynamic arrays, slices, `list`,
`map`, and `set` belong to `collections.md`.

This rulebook defines language semantics, not mutable implementation status.
Implementation progress belongs only in `implementation-status.yaml`.

MLIR operation schemas and lowering details belong under `rules/mlir/`. This
rulebook states the source semantics that those documents must preserve, but it
does not duplicate the detailed MLIR model.

---

# 2. Core model

A shaped value has:

```text
element type
rank
shape
logical element order
logical value semantics
semantic storage-placement contract
optional materialized storage
```

A shaped storage view additionally has or derives:

```text
backing storage identity
element origin or offset
strides
layout
memory space
borrow authority
```

Not every property occupies runtime storage.

Compile-time-known facts may be represented entirely in type metadata or
compiler analysis and erased when no runtime representation is required.

Sec distinguishes:

```text
logical shaped value
    what the value means

storage representation
    how an addressable representation is laid out

view
    a borrowed mapping over existing storage
```

The compiler may keep logical shaped values in SSA form or registers, scalarize
them, fuse operations, eliminate intermediates, and defer materialization until
ordinary language semantics require observable storage.

The compiler must not invent a fallible allocation, storage transfer, or
lifetime extension merely to satisfy an observation that the language does not
otherwise require.

---

# 3. Shaped type families

## 3.1 `vector`

```sec
vector[T, N]
```

is an owning rank-one shaped value.

Semantically:

```text
Rank  = 1
Shape = [N]
Len   = N
```

`N` is compile-time-known.

A vector is not a growable collection.

## 3.2 `matrix`

```sec
matrix[T, Rows, Columns]
```

is an owning rank-two shaped value.

Semantically:

```text
Rank  = 2
Shape = [Rows, Columns]
Len   = Rows * Columns
```

Both extents are compile-time-known.

## 3.3 Statically shaped `tensor`

```sec
tensor[T, D0, D1, ...]
```

is an owning shaped value whose rank and all extents are compile-time-known.

Example:

```sec
let image: tensor[float32, 3, 224, 224]
```

has:

```text
Rank  = 3
Shape = [3, 224, 224]
```

The variadic dimension form is compiler-known and does not imply general
variadic generic parameters for ordinary user-defined types.

## 3.4 Runtime-shaped owning `tensor`

```sec
tensor[T, Shape[Rank]]
```

is an owning tensor whose rank is compile-time-known and whose extents are
runtime values.

Example:

```sec
let image: tensor[float32, Shape[3]]
```

has:

```text
Rank
    compile-time-known as 3

Shape
    runtime Shape[3]

Len
    runtime product of Shape
```

The older rule that deferred runtime-shaped owning tensors is superseded.

Runtime-shaped owning tensors are part of the Sec language model. A particular
compiler release may report an explicit unsupported-feature diagnostic until
implementation is complete; it must not reinterpret the type as a view or an
ordinary dynamic array.

## 3.5 `tensor_view`

```sec
tensor_view[T, Rank]
```

is the compiler-known non-owning affine-strided shaped-view type form.

It is not an independently owning runtime value that grants backing access.
Actual safe source-level access uses:

```sec
ref tensor_view[T, Rank]
ref mut tensor_view[T, Rank]
```

Mutability authority comes from `ref` or `ref mut`, not from a separate mutable
view type.

Example:

```sec
fn ProcessImage(image: ref tensor_view[float32, 3]) void {
}

fn MutateImage(image: ref mut tensor_view[float32, 3]) void {
}
```

A view remains tied to its backing storage lifetime, provenance, generation,
invalidation domain, and memory-space contract.

---

# 4. `Shape[Rank]`

```sec
Shape[Rank]
```

is a compiler-known immutable nominal value containing one non-negative extent
per axis.

Rank is part of the type.

A `Shape[3]` is not interchangeable with `list[uint]` or `uint[3]` merely
because the representations could be similar.

Zero extents are valid.

A shaped value with at least one zero extent has:

```text
Len == 0
```

Negative extents are invalid.

The product of all extents must be checked for overflow whenever it is not
proven safe at compile time.

---

# 5. Rank, Shape, and Len

All shaped values and shaped views expose the semantic read-only properties:

```sec
value.Rank
value.Shape
value.Len
```

The canonical result types are:

```text
Rank  -> uint
Shape -> Shape[Rank]
Len   -> uint
```

`Len` is the total logical number of scalar elements:

```text
Len = product of all extents in Shape
```

It never means only the first dimension.

Examples:

```sec
let v: vector[float32, 4]
let m: matrix[float32, 3, 4]
let t: tensor[float32, 2, 3, 4]

v.Len // 4
m.Len // 12
t.Len // 24
```

For a broadcast view, `Len` remains the number of logical elements even when
multiple logical indexes refer to the same backing element.

`Rank`, `Shape`, and `Len` are compiler-known facts and may be folded when
statically known.

The returned `Shape` is read-only semantic metadata. Mutating a shaped value's
elements through `ref mut` does not grant permission to mutate its shape
metadata directly.

## 5.1 Type-level access

When the complete fact belongs to the static type, the corresponding associated
property is available.

Examples:

```sec
matrix[float32, 3, 4].Rank
matrix[float32, 3, 4].Shape
matrix[float32, 3, 4].Len

tensor_view[float32, 3].Rank
```

For:

```sec
tensor_view[float32, 3]
```

the type knows rank but not a particular runtime shape, so a complete associated
`.Shape` or `.Len` is not available from the type alone.

The same restriction applies to:

```sec
tensor[float32, Shape[3]]
```

for runtime extents.

---

# 6. Strides

```sec
Strides[Rank]
```

is a compiler-known immutable nominal value containing one element stride per
axis.

Stride values are measured in elements, not bytes.

Every affine-strided shaped value or view exposes:

```sec
value.Strides
```

as a read-only property.

For canonical dense owning values, strides are normally compile-time-known.
For runtime-shaped values or views, they may be runtime-derived.

A stride may be:

```text
positive
    forward traversal

negative
    reverse traversal

zero
    broadcasted axis
```

Zero and negative strides are valid view semantics.

## 6.1 Canonical dense row-major strides

The canonical ordinary owning layout is dense row-major unless an explicit
storage request requires another supported layout.

For:

```sec
matrix[T, Rows, Columns]
```

canonical element strides are:

```text
[Columns, 1]
```

For a rank-N dense row-major tensor:

```text
the last axis has stride 1
each preceding stride is the product of later extents
```

## 6.2 Address selection

For an affine-strided view, an element is conceptually selected by:

```text
origin + sum(index[axis] * stride[axis])
```

in element units.

The compiler must validate all required bounds, overflow, backing-range,
addressability, and memory-space facts before safe access is accepted.

---

# 7. Tensor layout

```sec
TensorLayout[Rank]
```

is a compiler-known nominal description of the shaped storage-layout category.

All shaped values and views for which layout is meaningful expose:

```sec
value.Layout
```

as a read-only property.

The canonical public affine-strided categories are:

```text
DenseRowMajor
DenseColumnMajor
Strided
```

`DenseRowMajor` and `DenseColumnMajor` are contiguous dense layouts with a
canonical axis order.

`Strided` represents an affine mapping that is not represented as one of those
canonical dense categories.

Additional source-visible layout categories may be defined by additive
rulebooks without changing the meaning of these categories or the affine
`Strides` contract defined here.

The compiler/backend may internally use tiled, blocked, sparse, swizzled, or
other representations when they are observationally equivalent to the source
contract. Such internal choices do not automatically become public
`TensorLayout` values.

Changing `.Layout` by property assignment is invalid.

---

# 8. Contiguity

All shaped values and views expose the read-only property:

```sec
value.IsContiguous
```

`IsContiguous` is intentionally symmetric across the shaped API even when the
answer is statically always `true` for a canonical dense owning value.

This symmetry allows generic shaped code to ask the same meaningful question
without branching on the concrete shaped family.

A shaped mapping is contiguous when its logical elements are represented by one
forward, dense, non-overlapping storage range in logical traversal order.

Consequences include:

```text
canonical dense owning vector/matrix/tensor
    true

canonical row view of row-major matrix
    normally true

column view of row-major matrix
    normally false

ordinary transpose view
    normally false

step greater than 1
    false

negative-stride reversal
    false

broadcast with zero stride
    false
```

A physically adjacent but reverse-traversed range is not contiguous under this
public definition.

This strict definition makes `IsContiguous` safe and useful for FFI and raw
buffer decisions.

---

# 9. Memory space

A shaped value or view with a storage-placement contract exposes:

```sec
value.MemorySpace
```

as a read-only property.

`MemorySpace` describes the semantic access and transfer domain of the value's
storage contract. It does not describe storage origin or allocation lifetime.

Examples of distinct concepts:

```text
StorageOrigin.Arena
    allocation/lifetime origin

MemorySpace ordinary or target-defined accelerator memory
    access/transfer domain
```

They are orthogonal.

A logical shaped value may have a semantic `MemorySpace` contract even while the
compiler temporarily keeps the value in SSA or registers.

Reading `.MemorySpace` does not by itself force allocation or materialization.

A view inherits the memory space of its backing storage.

A view cannot independently assign or change its memory space.

Changing memory space is an explicit storage-producing operation and is never a
property assignment.

---

# 10. Ptr and SizeOf

## 10.1 `Ptr`

Addressable shaped values and shaped views expose the compiler-known unsafe
read-only property:

```sec
value.Ptr
```

The result is a raw pointer to the logical origin element of the current
addressable representation.

For a view, `.Ptr` does not imply that subsequent logical elements are adjacent.
The caller must use `.IsContiguous`, `.Shape`, `.Strides`, `.Layout`, and the
foreign/unsafe contract as required.

Accessing `.Ptr` must not silently allocate merely to make a pure temporary
addressable.

## 10.2 `SizeOf`

Shaped values and views expose:

```sec
value.SizeOf
```

as the logical represented payload byte count.

Conceptually:

```text
value.SizeOf = value.Len * element payload size
```

with checked arithmetic and target layout rules.

For a non-contiguous or broadcast view, `.SizeOf` is not:

```text
the descriptor size
the byte span between minimum and maximum backing address
the count of unique backing bytes touched
```

A broadcast view may therefore have a large logical `.SizeOf` while referring
to one backing element repeatedly.

`Ptr + SizeOf` describes one directly usable contiguous payload buffer only when
the relevant layout and FFI requirements are satisfied and `.IsContiguous` is
true.

Associated type `.SizeOf` and global `SizeOf(T)` remain physical type-layout
queries under the compiler-known-member and layout rules.

---

# 11. Storage requests

## 11.1 General request

The general memory/storage rulebook owns:

```sec
struct StorageRequest {
    MemorySpace: Option[MemorySpace]
    MinAlignment: Option[uint]
}
```

The exact declaration may be compiler-known rather than ordinary source, but
these fields and semantics are normative.

An explicitly supplied field is a requirement, not a hint.

```text
MemorySpace: None
    no additional memory-space requirement

MemorySpace: Some(space)
    destination storage must use that space

MinAlignment: None
    use at least the natural required alignment

MinAlignment: Some(n)
    destination alignment must be at least n
```

`None` is not equivalent to explicitly requesting ordinary memory.

## 11.2 Shaped request

Shaped storage adds layout requirements by composition:

```sec
struct ShapedStorageRequest[Rank] {
    Storage: StorageRequest
    Layout: Option[TensorLayout[Rank]]
}
```

An explicitly supplied `Layout` is a hard destination requirement.

An omitted layout uses the operation's canonical valid layout. For ordinary
owned dense shaped construction that canonical layout is `DenseRowMajor`.

Allocation authority is not a field of `StorageRequest` or
`ShapedStorageRequest`.

An `Arena`, allocator, or equivalent allocation capability remains a separate
argument/capability with its own ownership and borrowing semantics.

---

# 12. Ordinary construction and explicit creation

Ordinary canonical value construction does not require or imply a source-level
parameterless `Create()` call.

Example:

```sec
let matrix: matrix[float32, 4, 4]
```

uses the type's canonical storage contract when storage is required.

The compiler must not model this source construct as if the programmer had
written a meaningless `.Create()` call.

Explicit `Create(...)` is reserved for active storage control, such as a
non-default request or explicit allocation authority.

Conceptual forms include:

```sec
let value := try matrix[float32, 4096, 4096].Create(request)

let value := try matrix[float32, 4096, 4096].Create(
    ref mut arena,
    request,
)
```

Storage-related fallibility must not be hidden by ordinary construction.

If a source operation semantically requires a fallible storage acquisition, the
fallibility must be explicit at that storage-producing boundary.

---

# 13. Storage-producing operations

## 13.1 `Materialize`

A view may create new owning storage from its logical contents:

```sec
let owned := try view.Materialize()
let placed := try view.Materialize(request)
```

For:

```sec
ref tensor_view[T, Rank]
```

the canonical runtime-shaped owning result is:

```sec
tensor[T, Shape[Rank]]
```

unless the programmer explicitly requests construction into a compatible
statically shaped destination type through that destination type's checked
conversion API.

`Materialize`:

```text
creates new owning storage
may allocate
copies/materializes logical elements
preserves logical element order
uses the requested destination storage contract
leaves the source view and backing unchanged
```

A non-contiguous or broadcast view may be packed into canonical dense storage by
materialization.

## 13.2 `TransferTo`

An owning shaped value may explicitly produce a new owning value under a
different storage request:

```sec
let deviceValue := try value.TransferTo(request)
```

`TransferTo`:

```text
creates a new owning destination
leaves the source owner valid
may allocate
may copy, map, use DMA, synchronize, or use target-specific transfer mechanisms
must satisfy the complete destination request
must not silently substitute an incompatible memory space
```

A successful synchronous `TransferTo` is complete on return.

After `Ok`, the destination is fully initialized and usable according to its
memory-space contract.

A genuinely asynchronous transfer must use a separate explicit asynchronous
API/handle that represents outstanding source/destination lifetime and
completion obligations. It must not reuse synchronous `TransferTo` semantics.

## 13.3 `Relayout`

An owning shaped value may request a new owning representation with a different
layout:

```sec
let columnMajor := try value.Relayout(TensorLayout[2].DenseColumnMajor)
```

`Relayout` may allocate and reorder elements.

It leaves the source owner valid.

An implementation may combine transfer and relayout work when the destination
request permits it. Source semantics describe the required destination
contract, not a mandatory sequence of physical implementation steps.

---

# 14. Reshape, ToShape, and Materialize

Sec distinguishes three operations deliberately.

## 14.1 `Reshape`

```sec
let view := try value.Reshape(newShape)
```

`Reshape` creates a non-owning view over the same backing storage.

It:

```text
does not allocate
does not copy elements
does not transfer ownership
preserves backing storage identity
preserves MemorySpace
requires equal logical element count
requires reshape-compatible contiguous logical storage
```

The result is:

```sec
ref tensor_view[T, NewRank]
```

or, when mutable authority may legally be preserved:

```sec
ref mut tensor_view[T, NewRank]
```

The result rank is the rank encoded by the supplied `Shape[NewRank]`.

When element-count compatibility is compile-time-proven, no runtime check or
`try` is required.

When compatibility depends on runtime shape values, the operation is checked and
uses `ShapeError`.

`Reshape` never hides a copy to make a non-contiguous input acceptable.

## 14.2 `ToShape`

```sec
let reshaped := try value.ToShape(newShape)
```

`ToShape` is the consuming owning reshape operation.

It:

```text
consumes the source owner
reuses the same backing storage
performs no element copy
performs no allocation
preserves MemorySpace
preserves storage identity
changes the owning shaped type/shape metadata
requires equal logical element count
```

Example:

```sec
let image: tensor[float32, Shape[3]]

let flat := try image.ToShape(
    Shape[1] { image.Len },
)

// image has been consumed
```

`ToShape` is prohibited when active borrows prevent consuming or reinterpreting
the owner.

The `To` spelling is canonical Sec naming and does not imply copying.

## 14.3 Why the operations are distinct

```text
Reshape
    same backing
    borrowed result
    no allocation
    no copy

ToShape
    same backing
    new owner
    consumes source
    no allocation
    no copy

Materialize
    new backing
    new owner
    may allocate
    copies/materializes logical contents
```

---

# 15. Axis values

## 15.1 `Axes[Rank]`

```sec
Axes[Rank]
```

is a compiler-known immutable axis-permutation value.

It contains every axis index in:

```text
0..<Rank
```

exactly once.

Example:

```sec
Axes[3] { 2, 0, 1 }
```

is valid.

Duplicate, missing, or out-of-range axes are invalid.

`Axes` is not itself a shaped value merely because it is an indexed fixed-size
metadata value. It does not gain meaningless `Shape`, `Strides`, `Layout`, or
`MemorySpace` members.

## 15.2 `AxisList[Count]`

```sec
AxisList[Count]
```

is a compiler-known immutable ordered list of distinct axis indexes used when an
operation selects only part of a rank.

Example:

```sec
AxisList[2] { 1, 3 }
```

Rules:

```text
every axis must be in range for the consuming operation
no axis may occur twice
order is semantically significant
```

`AxisList` is not required to contain every axis.

---

# 16. Permutation and transpose

## 16.1 `Permute`

```sec
let view := value.Permute(
    Axes[3] { 2, 0, 1 },
)
```

`Permute` is a non-owning view transformation.

It:

```text
uses the same backing storage
does not allocate
does not copy elements
preserves MemorySpace
permutes Shape
permutes Strides
recomputes Layout classification
recomputes IsContiguous
```

Conceptually:

```text
result.Shape[i]   = source.Shape[axes[i]]
result.Strides[i] = source.Strides[axes[i]]
```

Mutability follows ordinary borrow and overlap rules.

## 16.2 `Transpose`

For rank two:

```sec
let transposed := matrix.Transpose()
```

is the convenience form of:

```sec
matrix.Permute(Axes[2] { 1, 0 })
```

`Transpose()` is a view operation and does not physically reorder backing
storage.

Physical reordered ownership is obtained explicitly, for example:

```sec
let packed := try matrix.Transpose().Materialize()
```

---

# 17. Indexing and slicing

## 17.1 One selector per axis

A multidimensional shaped index expression supplies exactly one selector for
each source axis.

For a rank-three tensor:

```sec
value[i, j, k]
value[i, .., k]
value[.., .., ..]
```

are structurally valid selector counts.

Sec does not implicitly append omitted trailing `..` selectors.

Core shaped slicing has no ellipsis selector.

## 17.2 Scalar selector

A scalar index removes that axis from the result.

If every selector is scalar:

```sec
value[i, j, k]
```

the result is a `Place[T]` according to ordinary Place and mutability rules.

Sec does not create a rank-zero tensor for full scalar indexing.

## 17.3 Range selector

A range selector preserves the axis.

Examples:

```sec
matrix[2, ..]
matrix[.., 2]
tensor[1, .., 2]
tensor[1..<3, .., 4..<10]
```

When at least one axis remains, the result is:

```sec
ref tensor_view[T, ResultRank]
```

or:

```sec
ref mut tensor_view[T, ResultRank]
```

when the source authority and resulting mapping permit mutable borrowing.

Slicing:

```text
does not allocate
does not copy
preserves backing identity
preserves MemorySpace
derives new Shape and Strides
recomputes IsContiguous
```

The result view type is rank-parametrized, not extent-parametrized. Even when a
slice result shape is fully compile-time-known, its type remains:

```sec
ref tensor_view[T, ResultRank]
```

with the static shape retained as compiler-known metadata.

## 17.4 Ranges

Shaped slicing reuses ordinary Sec range syntax:

```sec
start..end
start..<end
..
start..
..<end
```

The existing inclusive/exclusive endpoint rules remain unchanged.

Internal normalization may use half-open `[start, endExclusive)` form.

---

# 18. Step and reversed views

A range selector may use the existing Sec `step` syntax:

```sec
value[.. step 2]
value[10..<90 step 2]
value[.. step -1]
```

No tensor-specific colon slicing language is introduced.

The step must be non-zero.

```text
step > 0
    forward strided view

step < 0
    reverse strided view

step == 0
    invalid
```

Zero stride remains reserved for broadcast semantics and is not created by
`step 0`.

For a selected axis:

```text
resultStride = sourceStride * step
```

subject to checked arithmetic.

For negative steps, inclusive/exclusive end semantics remain the ordinary Sec
range semantics; direction does not invert the meaning of `..` versus `..<`.

Open bounds are direction-sensitive:

```text
positive step
    omitted start -> first logical element
    omitted end   -> end boundary

negative step
    omitted start -> last logical element
    omitted end   -> before-start boundary
```

Therefore:

```sec
value[.. step -1]
```

is the canonical zero-copy reversed view of one axis.

A separate shaped `Reverse(axis)` view operation is not required by the core
language semantics.

---

# 19. Aliasing and mutable views

A safe `ref mut tensor_view[T, Rank]` must not expose overlapping logical indexes
that can mutate the same backing element through the same exclusive view.

Negative strides do not inherently forbid mutable views. A reversed view may
remain mutable when every logical index maps to a unique backing element.

Zero-stride broadcast expansion creates overlapping logical indexes and removes
mutable authority from the result.

This is a source-level rule, not merely an optimizer observation.

---

# 20. Broadcasting

## 20.1 Scalar operations

Scalar-to-shaped arithmetic is direct shaped arithmetic and does not require a
broadcast view.

Examples:

```sec
let scaled := image * 0.5g
let shifted := image + biasValue
```

A scalar has no shaped axes that require axis matching.

## 20.2 Explicit shaped broadcasting

Different shaped values are not implicitly broadcast for shaped arithmetic.

The programmer requests shaped broadcasting explicitly:

```sec
let expanded := right.BroadcastTo(left.Shape)
let result := left + expanded
```

`BroadcastTo(targetShape)` uses canonical right-aligned matching.

Example:

```text
source:       [   3, 1]
destination:  [2, 3, 4]
```

is valid.

Matching proceeds from trailing axes toward leading axes.

For each aligned source axis:

```text
source extent == destination extent
    preserve the source stride

source extent == 1
    expansion is allowed and destination stride becomes 0
```

Missing leading source axes behave as extent 1 and use zero stride in the
expanded view.

Any other extent mismatch is invalid.

Broadcasting:

```text
does not allocate
does not copy
preserves backing identity
preserves MemorySpace
```

A real expansion that creates zero-stride aliasing produces a shared:

```sec
ref tensor_view[T, ResultRank]
```

even when the source had mutable authority.

An identity/no-expansion broadcast may preserve `ref mut` authority when normal
borrow rules permit it.

An additive explicit axis-mapping API may be defined separately without changing
the meaning of canonical right-aligned `BroadcastTo(targetShape)`.

---

# 21. Elementwise arithmetic

For shaped values, ordinary arithmetic operators retain ordinary scalar
semantics and use elementwise shaped semantics when both operands are shaped.

Canonical elementwise operators include, where valid for the element type:

```sec
left + right
left - right
left * right
left / right
```

Two shaped operands require exactly compatible shapes.

Sec does not perform implicit shaped broadcasting.

Example:

```sec
let a: matrix[float32, 3, 4]
let b: matrix[float32, 1, 4]

let expanded := b.BroadcastTo(a.Shape)
let c := a + expanded
```

Scalar forms are direct and ergonomic:

```sec
matrix * scalar
scalar * matrix
matrix / scalar
matrix + scalar
scalar + matrix
matrix - scalar
```

The result element type follows ordinary scalar operator and unit algebra.

## 21.1 Logical result and allocation

Shaped arithmetic produces a logical shaped value.

Arithmetic itself is not fallible merely because a later materialized result
might require allocation.

Examples do not acquire allocation-related `try`:

```sec
let c := a + b
let d := a * b
```

The compiler may fuse, scalarize, keep intermediates in SSA/registers, reuse
storage when semantically legal, or otherwise optimize the logical result.

If a later ownership/storage boundary genuinely requires a fallible storage
resource, that fallibility must be explicit at that boundary.

The compiler must not hide a required fallible materialization.

## 21.2 Runtime shape checks

If shape compatibility is compile-time-proven, no `try` is required.

If incompatibility is compile-time-proven, compilation fails.

If compatibility depends on runtime shape values, the shaped operation is
checked and uses `ShapeError`, so `try` is required.

Example:

```sec
let a: tensor[float32, Shape[3]]
let b: tensor[float32, Shape[3]]

let c := try a + b
```

The `try` is for the runtime shape check, never for speculative allocation by
the arithmetic expression.

---

# 22. Algebraic multiplication operator `x`

Sec uses contextual infix:

```sec
x
```

for canonical algebraic multiplication, especially matrix multiplication.

`*` remains elementwise/scalar multiplication.

Example:

```sec
let elementwise := left * right
let product := left x right
```

`x` is contextual and remains a valid ordinary identifier outside operator
position:

```sec
let x := 10
```

The formatter requires one space on each side of operator `x`.

`x` has multiplicative precedence with `*`, `/`, and `%`, and is
left-associative.

## 22.1 Matrix by matrix

```sec
matrix[L, Rows, Inner] x matrix[R, Inner, Columns]
```

produces:

```sec
matrix[ProductElement[L, R], Rows, Columns]
```

where the scalar product type is derived from ordinary element multiplication
and the product terms support compatible addition for accumulation.

## 22.2 Matrix by vector

A plain `vector[T, N]` is treated as a column vector for matrix multiplication.

```sec
matrix[L, Rows, Columns] x vector[R, Columns]
```

produces:

```sec
vector[ProductElement[L, R], Rows]
```

## 22.3 No hidden interpretation

The core `x` operator is not general tensor contraction and does not implicitly
choose tensor axes.

The following are not given a hidden meaning by this rulebook:

```sec
vector[T, N] x vector[T, N]
vector[T, N] x matrix[T, N, M]
tensor[T, ...] x tensor[T, ...]
matrix[T, R, C] x scalar
scalar x matrix[T, R, C]
```

Use the explicit operation that expresses the intended algebra, such as:

```sec
left.Dot(right)
left.Outer(right)
left.Contract(right, leftAxes, rightAxes)
matrix * scalar
```

## 22.4 Runtime dimensions

Matrix or other canonical `x` forms with runtime-known extents follow the same
static-proof rule as elementwise arithmetic:

```text
provably valid
    no try

provably invalid
    compile-time error

runtime-dependent
    checked ShapeError and try
```

---

# 23. Memory-space compatibility of arithmetic

A shaped binary arithmetic operation does not implicitly transfer storage
between memory spaces.

When both operands carry shaped storage-placement contracts, the operation must
be legal under compatible `MemorySpace` contracts.

The compiler must not silently copy RAM to accelerator memory, accelerator
memory to RAM, or between distinct device spaces merely to make an operator
expression legal.

An explicit transfer is required first when the spaces are incompatible.

Scalar operands do not contribute a shaped memory-space contract.

A logical arithmetic result inherits the applicable shaped storage-placement
contract until an explicit destination request changes it.

---

# 24. Vector algebra

The following canonical vector methods are compiler-known shaped operations
when their element algebra is valid.

## 24.1 `Dot`

```sec
let result := left.Dot(right)
```

For two compatible vectors of the same length, `Dot` returns a scalar formed by
multiplying corresponding elements and accumulating the products.

It does not return a rank-zero tensor.

## 24.2 `Outer`

```sec
let result := left.Outer(right)
```

For:

```sec
vector[L, N]
vector[R, M]
```

it produces:

```sec
matrix[ProductElement[L, R], N, M]
```

as a logical shaped result.

## 24.3 `Magnitude`

```sec
let length := value.Magnitude()
```

is available only when the element type supplies the required multiplication,
addition, and square-root semantics.

Units participate in ordinary Sec unit algebra.

## 24.4 `Normalize`

```sec
value.Normalize()
```

returns:

```sec
Option[vector[...]]
```

A zero-magnitude vector returns `None`.

This is a normal domain case, not a `Result` error.

## 24.5 `Cross`

```sec
left.Cross(right)
```

is the canonical cross product for compatible rank-one vectors with extent 3.

It returns a compatible extent-3 vector whose element type follows ordinary
multiply/subtract algebra.

`Cross` is not generalized to arbitrary dimensions by hidden rules.

---

# 25. General tensor contraction

General tensor contraction is explicit:

```sec
let result := left.Contract(
    right,
    leftAxes,
    rightAxes,
)
```

where both axis arguments are `AxisList[Count]` values of equal `Count`.

Axes pair positionally.

Example:

```sec
let result := left.Contract(
    right,
    AxisList[2] { 1, 3 },
    AxisList[2] { 0, 2 },
)
```

pairs:

```text
left axis 1 with right axis 0
left axis 3 with right axis 2
```

Each paired extent must match exactly.

No implicit broadcasting occurs during contraction.

Result axis order is:

```text
all uncontracted left axes in original order
followed by
all uncontracted right axes in original order
```

Result rank is:

```text
left.Rank + right.Rank - 2 * Count
```

Result categories:

```text
rank 0
    scalar

rank >= 1, all extents static
    statically shaped tensor

rank >= 1, any result extent runtime
    tensor[T, Shape[ResultRank]]
```

The general `Contract` operation does not dynamically switch to `vector` or
`matrix` result families merely because the resulting rank is one or two.

`Dot`, `Outer`, and `x` remain the ergonomic canonical specialized operations.

Static and runtime validity follow the ordinary shaped proof/check rules.

---

# 26. Iteration

Shaped iteration follows logical index order, not accidental physical address
order.

Canonical logical traversal is lexicographic by axis with the last axis varying
fastest.

A view with negative strides, permutation, or other non-canonical physical
mapping still iterates according to its logical shape and axis order.

A broadcast view may therefore yield the same backing element at several
logical indexes.

Mutation during iteration is permitted only when ordinary borrow, alias,
overlap, and storage-stability rules permit it.

No iteration rule grants mutable access through an overlapping shared broadcast
view.

Axis-oriented iterators or higher-level numerical iteration helpers may be added
by ordinary/core library APIs without changing this logical traversal contract.

---

# 27. Ownership, copying, moving, and destruction

`vector`, `matrix`, and `tensor` are owning shaped families.

`tensor_view` is non-owning.

`Shape`, `Strides`, `TensorLayout`, `Axes`, `AxisList`, `MemorySpace`, and
storage-request values are metadata/descriptor-like nominal values according to
their owning rulebooks.

Copyability of an owning shaped value follows the ordinary Sec copy/move rules,
element type, storage representation, and active borrow state.

The compiler must not invent an implicit deep copy merely because a shaped value
has owning backing storage.

A move transfers ownership according to the ordinary ownership model.

`ToShape` is explicitly consuming even though it performs no element copy.

Each initialized element owned by an owning shaped value must be destroyed
exactly once.

Views never destroy backing elements.

---

# 28. No hidden heap

No shaped type assumes a hidden global heap.

Any storage-producing operation that requires allocation must use a valid
allocation context/provider such as:

```text
Arena
explicit allocator
target/device provider
other approved allocation authority
```

An explicitly requested `MemorySpace` constrains allocation-provider selection.
The compiler must select a provider capable of satisfying the requested space or
report the operation as unsupported/fallible according to the storage contract.

Escape analysis must not repair an invalid shaped borrow or view escape by
silently materializing or deep-copying it.

---

# 29. Error model

Shaped operations use a small consistent error model.

## 29.1 Compile-time rejection

When invalidity is statically proven, compilation fails.

Examples include:

```text
static out-of-bounds index
invalid static range
step 0 known at compile time
invalid static axis index
duplicate axis in a compile-time Axes/AxisList value
static shape mismatch
static invalid reshape
static invalid contraction
```

## 29.2 `Option`

`Option[T]` is used when the operation is semantically valid but there may be no
result for a normal domain case and no error reason is required.

Canonical shaped example:

```sec
value.Normalize() -> Option[vector[...]]
```

A zero vector yields `None`.

## 29.3 `ShapeError`

Runtime-dependent shape validity uses `ShapeError`.

The canonical family must cover at least:

```text
RankMismatch
DimensionMismatch
ElementCountOverflow
InvalidReshape
InvalidBroadcast
InvalidContraction
InvalidAxis
DuplicateAxis
InvalidStep
```

Implementations may use more precise internal reasons while preserving the
public error contract.

## 29.4 Index and range errors

Scalar indexing uses `IndexError` where runtime checking is required.

Range/slicing validity uses `RangeError` where runtime checking is required.

## 29.5 `StorageError`

Explicit shaped storage-producing operations use `StorageError` when the
requested destination contract or physical operation can fail.

The public family must be able to represent at least:

```text
AllocationFailed
UnsupportedMemorySpace
UnsupportedLayout
AlignmentUnsatisfied
TransferFailed
SizeOverflow
```

The underlying allocator may use `AllocationError`; shaped/storage APIs map
provider-specific failures to the public storage-facing contract where required.

An explicit `Err` return is not itself an effect.

---

# 30. Static proof and `try`

Sec uses static proof first.

For shaped validity:

```text
provably valid
    operation is infallible for that condition
    no try is required

provably invalid
    compile-time diagnostic

runtime-dependent
    checked operation
    try is required when the operation returns Result
```

This applies to:

```text
runtime-shaped elementwise arithmetic
runtime matrix multiplication
Reshape
ToShape
BroadcastTo
Contract
runtime indexing and slicing where applicable
```

`try` on shaped arithmetic is for a real runtime semantic check such as shape
compatibility. It is never inserted merely because a logical arithmetic result
might eventually need materialized storage.

---

# 31. Sema requirements

Sema must track enough information to validate shaped semantics without
reconstructing them from backend pointer arithmetic.

At minimum it must know or conservatively represent:

```text
ElementType
Rank
Shape
static versus runtime shape facts
Strides
Layout
MemorySpace
IsContiguous
ownership state
borrow authority
backing identity/dependency
```

Sema must validate at least:

```text
shaped type forms and arity
static and runtime extents
element-count overflow
storage eligibility
Rank/Shape/Len properties
layout and stride validity
index selector count
index and range bounds
step validity
view overlap and mutable authority
Reshape and ToShape compatibility
Axes and AxisList validity
Permute and Transpose
BroadcastTo compatibility
operator shape compatibility
x matrix/matrix and matrix/vector algebra
Dot, Outer, Magnitude, Normalize, Cross
Contract axis and extent compatibility
MemorySpace compatibility
StorageRequest and ShapedStorageRequest requirements
Materialize, TransferTo, and Relayout ownership/effect boundaries
Ptr addressability
```

Shaped Place/projection analysis must remain compatible with ordinary Sec
borrowing and lifetime analysis.

---

# 32. Effects

Logical shaped operations do not acquire effects merely because a future
materialization could allocate.

The following are non-allocating by source semantics:

```text
Rank / Shape / Len / Strides / Layout / MemorySpace / IsContiguous queries
index/view formation when checks themselves need no external resource
Reshape
ToShape
Permute
Transpose
BroadcastTo
ordinary logical shaped arithmetic
Dot
Outer
Magnitude
Normalize
Cross
Contract
```

`Materialize`, explicit `Create`, `TransferTo`, and `Relayout` are
storage-producing operations and contribute the actual provider/helper effects
required by the selected implementation.

Examples may include:

```text
MayAllocate
MayBlock
MaySuspend
MayIO
```

A synchronous `TransferTo` must not return success while an unrepresented
background transfer still depends on source storage.

The detailed effect lattice remains owned by `effect_analysis.md`.

---

# 33. LSP and tooling requirements

LSP completion and hover must use the same compiler-known shaped member registry
and Sema facts as compilation.

Hover must clearly display:

```text
resolved shaped type
Rank
Shape when useful
Len when useful
static versus runtime status when useful
exact method/property return type
required try when runtime checking is required
Option versus Result meaning
MemorySpace/Layout/IsContiguous facts when they materially affect use
unsafe requirement for Ptr
```

For `Normalize`, hover should make the domain absence explicit, for example:

```text
Normalize() -> Option[vector[float32, 3]]
Returns None when the vector has zero magnitude.
```

For a storage-producing operation, hover should show the exact `Result` error
contract and concise storage-failure meaning.

The LSP must not invent a separate shaped API table independent from Sema.

Formatter support must preserve the canonical contextual `x` operator and
ordinary Sec `step` syntax.

---

# 34. Compiler/core/standard-library boundary

The shaped families are compiler-known first-class language types.

The compiler owns their:

```text
type identity
source type forms
canonical intrinsic properties
canonical shaped operations
operator semantics
validation rules
ownership/borrow-sensitive semantics
required compiler-known member identities
```

A core helper or standard-library algorithm may implement reusable behavior, but
it does not own or redefine the semantics listed in this rulebook.

Ordinary user modules may not globally monkey-patch or add privileged `impl`
blocks to compiler-owned shaped types.

The standard library may provide additive higher-level numerical algorithms that
do not alter the canonical shaped language surface.

Examples include decompositions, eigensolvers, domain-specific reductions, and
specialized numerical algorithms.

No stdlib implementation is required merely to make a compiler-known property
such as `.Rank`, `.Shape`, `.Len`, `.Strides`, `.IsContiguous`, `.Ptr`, or
`.SizeOf` semantically exist.

---

# 35. MLIR ownership of lowering details

The canonical detailed IR and lowering model for shaped operations belongs under:

```text
rules/mlir/
```

In particular, the MLIR rulebooks own the detailed operation schemas,
verification, bufferization, memory-space lowering, target mapping, and
optimization constraints.

Those documents must preserve the source distinctions defined here, including:

```text
logical value versus view
Reshape versus ToShape versus Materialize
explicit MemorySpace transfer
layout requirements
zero-stride broadcast aliasing
non-allocating logical arithmetic
x versus elementwise *
runtime shape checks
ownership and borrow authority
```

Backend lowering must not reinterpret these source semantics.

---

# 36. Diagnostics

Diagnostics must have stable IDs and should state the violated shaped contract,
not merely report a backend type mismatch.

Examples:

```text
matrix multiplication requires matching inner dimensions, got 4 and 5
```

```text
elementwise multiplication requires identical shapes, got [3, 4] and [4, 3]
```

```text
broadcast target [2, 3, 4] is incompatible with source shape [3, 2]
```

```text
mutable broadcast view would alias backing storage through zero stride on axis 0
```

```text
reshape requires 24 elements but target shape contains 30
```

```text
step must not be zero
```

```text
requested memory space is not supported by the active allocation provider
```

Diagnostics should identify whether a failure is:

```text
compile-time-proven invalidity
runtime check requiring try
storage request failure
borrow/ownership restriction
unsafe Ptr requirement
```

---

# 37. Conformance requirements

A conforming implementation must test at least:

```text
vector, matrix, static tensor, runtime-shaped tensor identities
shared and mutable tensor_view forms
Rank, Shape, Len
Strides, Layout, MemorySpace, IsContiguous
Ptr addressability and unsafe requirements
SizeOf logical payload semantics
static and runtime shape checks
indexing and multidimensional slicing
positive and negative step
broadcast and broadcast mutability
Reshape
ToShape ownership consumption
Materialize
TransferTo
Relayout
Axes and AxisList
Permute and Transpose
elementwise + - * /
scalar-shaped arithmetic
x matrix multiplication
Dot, Outer, Magnitude, Normalize, Cross
Contract
same-space arithmetic rejection for incompatible spaces
StorageRequest and ShapedStorageRequest
no hidden allocation in logical arithmetic
formatter and LSP display
```

Invalid tests must document the expected diagnostic and reason.

---

# 38. Canonical summary

```text
vector[T, N]
    owning static rank-1 shaped value

matrix[T, R, C]
    owning static rank-2 shaped value

tensor[T, D0, D1, ...]
    owning statically shaped tensor

tensor[T, Shape[Rank]]
    owning runtime-shaped tensor with static rank

tensor_view[T, Rank]
    non-owning view type form

ref tensor_view[T, Rank]
    shared safe shaped view

ref mut tensor_view[T, Rank]
    exclusive mutable shaped view when mapping is non-overlapping

Rank / Shape / Len
    logical shaped properties

Strides / Layout / MemorySpace / IsContiguous
    meaningful shaped storage properties
    read-only

Ptr
    unsafe read-only address observation

SizeOf
    logical represented payload byte count on shaped instances

StorageRequest
    hard general destination-storage requirements

ShapedStorageRequest
    StorageRequest plus shaped layout requirement

ordinary construction
    uses canonical storage semantics without parameterless Create()

Create(request)
    explicit initial storage control

Reshape
    borrowed zero-copy shape reinterpretation

ToShape
    consuming zero-copy owning shape reinterpretation

Materialize
    new owning backing from a view

TransferTo
    explicit new owning destination under a storage request

Relayout
    explicit new owning representation with another layout

Axes
    complete axis permutation

AxisList
    ordered distinct axis subset

Permute / Transpose
    zero-copy view transformations

multidimensional slicing
    one selector per source axis
    scalar selector removes axis
    range selector preserves axis

step
    reused for shaped strided and reversed views
    zero is invalid

BroadcastTo
    explicit right-aligned shaped broadcasting
    real expansion creates shared overlapping view

*
    elementwise/scalar multiplication

x
    canonical algebraic multiplication, especially matrix multiplication

Contract
    explicit general tensor contraction

logical shaped arithmetic
    not allocation-fallible

runtime shape uncertainty
    checked with ShapeError and try

MemorySpace transfer
    explicit; never hidden in ordinary arithmetic or assignment
```
