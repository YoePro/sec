# Normative correction — shaped compiler/core/stdlib boundary

**Target:** `rules/library/core-library.md`  
**Status:** Applied
**Applied:** 2026-08-13
**Language version:** Sec 0.1  
**Created:** 2026-08-13  
**Last updated:** 2026-08-13  
**Source authority:** `rules/collections/shaped-types.md`, `rules/compiler/compiler_known_members.md`

## Canonical boundary

`vector`, `matrix`, `tensor`, and `tensor_view` are compiler-known first-class
language type families.

Their canonical intrinsic properties and shaped operations exist semantically
because the compiler-known registry and owning shaped rulebook define them.

They do not require ordinary privileged core/stdlib `impl` declarations merely
to exist as language members.

Core or standard-library source may provide implementation helpers or additive
higher-level numerical algorithms, but such helpers do not own or redefine the
canonical shaped semantics.

Ordinary user modules may not monkey-patch compiler-owned shaped types with
privileged global `impl` blocks.

## Required shaped intrinsic surface

The core/library boundary must recognize compiler ownership of at least:

```text
Rank
Shape
Len
Strides
Layout
MemorySpace
IsContiguous
Ptr
SizeOf
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

and contextual operator `x` semantics.

## Additive standard-library scope

The standard library may add ordinary numerical algorithms such as:

```text
decomposition
eigensolvers
specialized reductions
domain-specific transforms
specialized sparse algorithms
```

when those APIs do not change the meaning of the compiler-known shaped surface.

## Legacy lowercase note

Any remaining core-library examples using legacy `.len` or `.ptr` spellings are
not authoritative over the canonical `.Len` and `.Ptr` compiler-known spellings.
The existing compiler-known-member migration rule remains controlling.
