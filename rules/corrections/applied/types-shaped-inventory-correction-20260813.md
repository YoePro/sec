# Normative correction — shaped type inventory

**Target:** `rules/types/types.md`  
**Status:** Applied
**Applied:** 2026-08-13
**Language version:** Sec 0.1  
**Created:** 2026-08-13  
**Last updated:** 2026-08-13  
**Source authority:** `rules/collections/shaped-types.md`

## Required shaped forms

The type inventory must include both owning tensor forms:

```sec
tensor[T, D0, D1, ...]
tensor[T, Shape[Rank]]
```

The first form has compile-time-known rank and extents.

The second form is an owning runtime-shaped tensor with compile-time-known rank
and runtime extents.

The old classification of runtime-shaped owning tensors as a deferred language
extension is superseded.

`tensor_view[T, Rank]` remains a compiler-known non-owning view type form.
Actual safe source-level views use:

```sec
ref tensor_view[T, Rank]
ref mut tensor_view[T, Rank]
```

## Required compiler-known support types

Add to the shaped/supporting inventory:

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

These support types are nominal semantic types and are not aliases for ordinary
arrays/lists merely because a representation could be similar.
