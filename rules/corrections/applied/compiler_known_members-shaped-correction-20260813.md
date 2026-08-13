# Normative correction — shaped compiler-known members

**Target:** `rules/compiler/compiler_known_members.md`  
**Status:** Applied
**Applied:** 2026-08-13
**Language version:** Sec 0.1  
**Created:** 2026-08-13  
**Last updated:** 2026-08-13  
**Source authority:** `rules/collections/shaped-types.md`

## Merge instruction

Merge this correction into the canonical compiler-known member registry. Do not
create an independent shaped-member table in Sema or LSP.

## Required shaped receiver families

The compiler-known registry must recognize the shaped families:

```text
vector[T, N]
matrix[T, Rows, Columns]
tensor[T, Dims...]
tensor[T, Shape[Rank]]
ref tensor_view[T, Rank]
ref mut tensor_view[T, Rank]
```

and the compiler-known support types:

```text
Shape[Rank]
Strides[Rank]
TensorLayout[Rank]
Axes[Rank]
AxisList[Count]
MemorySpace
StorageRequest
ShapedStorageRequest[Rank]
```

## Required read-only shaped properties

Where semantically meaningful, the registry must expose:

```text
Rank
Shape
Len
Strides
Layout
MemorySpace
IsContiguous
SizeOf
```

`Ptr` is an unsafe read-only property and additionally requires addressability.

`Rank`, `Shape`, `Len`, `Strides`, `Layout`, `MemorySpace`, and
`IsContiguous` are properties, not zero-argument methods.

`Len` on a shaped value means total logical scalar element count, not first-axis
extent.

`IsContiguous` is intentionally available across the shaped family for generic
symmetry. It may be compile-time `true` for canonical dense owning values.

## `SizeOf` on shaped instances

For shaped instances:

```text
value.SizeOf
    logical represented payload byte count
    conceptually value.Len * element payload size
```

It is not descriptor size, storage span, or unique backing-byte count for a
strided/broadcast view.

Associated `Type.SizeOf` and global `SizeOf(Type)` remain physical type-layout
queries.

## Required shaped operations

The canonical shaped API must resolve through compiler-known semantic identities
for at least:

```text
Reshape
ToShape
Materialize
TransferTo
Relayout
Permute
Transpose
BroadcastTo
Dot
Outer
Magnitude
Normalize
Cross
Contract
```

The `x` operator remains compiler-known algebraic shaped multiplication.

Compiler-known ownership/effect facts must distinguish:

```text
Reshape
    borrowed, non-allocating

ToShape
    consuming, non-allocating, same backing

Materialize
    new owner, storage-producing

TransferTo
    new owner, storage-producing, source remains valid

Relayout
    new owner, storage-producing when required
```

## Type-level properties

Associated `Rank`, `Shape`, and `Len` are available only when the complete fact
belongs to the static type.

For example:

```sec
matrix[float32, 3, 4].Rank
matrix[float32, 3, 4].Shape
matrix[float32, 3, 4].Len

tensor_view[float32, 3].Rank
```

A runtime-shaped `tensor[T, Shape[Rank]]` or `tensor_view[T, Rank]` does not
synthesize a type-level runtime `.Shape` or `.Len`.

## LSP integration

Completion, hover, go-to-definition, overload resolution, and diagnostics must
consume the same registry entries and Sema facts. No LSP-local shaped member
inventory is permitted.
