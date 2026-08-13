# Normative correction — source-level memory-space requests for shaped storage

**Target:** `rules/memory/storage.md`  
**Status:** Applied
**Applied:** 2026-08-13
**Language version:** Sec 0.1  
**Created:** 2026-08-13  
**Last updated:** 2026-08-13  
**Source authority:** `rules/collections/shaped-types.md`

## Superseded limitation

The previous statements that Sec 0.1 does not require first-class memory-space
contracts or general source-level memory-space/placement semantics are
superseded to the extent described below.

This correction does not introduce a `storage` keyword, source-level regions,
or lifetime parameters.

## Orthogonality remains canonical

`MemorySpace` remains orthogonal to:

```text
StorageOrigin
BackingRelation
ReclamationAuthority
AddressStability
```

`Arena`, automatic storage, static storage, and allocator-backed storage are not
memory-space values.

A target-defined accelerator/device memory space is not a storage origin.

## Source-level `MemorySpace`

`MemorySpace` is a compiler-known nominal storage descriptor identifying an
access/transfer domain.

The existing canonical categories remain:

```text
Ordinary
MMIO
TargetDefined
```

Target-defined concrete spaces may represent GPU, accelerator, DSP, secure,
non-volatile, foreign, or other target-specific domains according to their
compiler-known contracts.

## `StorageRequest`

The language model includes the compiler-known destination request:

```sec
struct StorageRequest {
    MemorySpace: Option[MemorySpace]
    MinAlignment: Option[uint]
}
```

An explicitly supplied field is a hard requirement, not a performance hint.

`None` means no additional requirement for that dimension. It is not equivalent
to explicitly requesting `Ordinary` memory.

`MinAlignment: Some(n)` requires actual destination alignment of at least `n`.
A provider may choose a larger valid alignment.

Allocation authority such as `ref mut Arena` is not stored inside
`StorageRequest`.

## Allocation-provider selection

When an allocating/storage-producing operation carries an explicit
`MemorySpace`, allocation-context resolution must select a provider capable of
satisfying that memory-space contract.

The compiler must not silently replace the requested memory space with another
space.

## Explicit transfer

Transfers between incompatible memory spaces remain explicit and may be
fallible.

A synchronous storage transfer used by shaped `TransferTo(...)` is complete on
successful return. The destination is fully initialized and usable under its
memory-space contract.

A genuinely asynchronous DMA/device transfer requires a distinct explicit API or
handle that represents outstanding source/destination lifetime, completion,
publication, and cancellation obligations.

Synchronous `TransferTo` must not return success while an unrepresented
background operation still depends on the source backing.

## Read-only observation

Public `.MemorySpace` observation is read-only.

Changing memory space is not property assignment. It requires an explicit
storage-producing operation.
