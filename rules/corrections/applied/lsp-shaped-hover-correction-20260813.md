# Normative correction — shaped hover and Option/Result presentation

**Target:** `rules/tooling/lsp.md`  
**Status:** Applied
**Applied:** 2026-08-13
**Language version:** Sec 0.1  
**Created:** 2026-08-13  
**Last updated:** 2026-08-13  
**Source authority:** `rules/collections/shaped-types.md`

## Shaped member source

LSP completion and hover must consume the compiler-owned shaped member registry
and Sema facts. The LSP must not maintain a second shaped API table.

## Required hover information

For a shaped expression or resolved shaped member, hover must present the exact
resolved type and exact callable/property return type.

When useful for correct programming, hover should additionally expose:

```text
Rank
Shape
Len
static versus runtime shape knowledge
IsContiguous
Layout
MemorySpace
unsafe Ptr requirement
runtime shape check requiring try
```

Nice-to-have analysis information remains governed by the existing configurable
hover-depth rules.

## Option versus Result

Hover must make the semantic distinction between `Option` and `Result` clear
when the distinction is needed to use the API correctly.

Canonical example:

```text
Normalize() -> Option[vector[float32, 3]]
Returns None when the vector has zero magnitude.
```

A storage-producing operation should show its exact error type and concise
principal failure condition, for example:

```text
Materialize(...) -> Result[tensor[...], StorageError]
May fail when the requested destination storage contract cannot be satisfied.
```

## Runtime shape checks

When a shaped operation requires `try` only because compatibility is not
statically proven, hover should state the runtime condition rather than imply
allocation fallibility.

Example intent:

```text
operator + -> Result[tensor[float32, Shape[3]], ShapeError]
Runtime shape equality must be checked.
```

When Sema proves compatibility, hover must show the infallible result instead.
