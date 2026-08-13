# Normative anchor — shaped operations and effect analysis

**Target:** `rules/analysis/effect_analysis.md`  
**Status:** Applied
**Applied:** 2026-08-13
**Language version:** Sec 0.1  
**Created:** 2026-08-13  
**Last updated:** 2026-08-13  
**Source authority:** `rules/collections/shaped-types.md`

## Required anchor

Effect analysis must preserve the distinction between logical shaped operations
and explicit storage-producing operations.

The following operations do not contribute `MayAllocate` merely because a later
materialization could require storage:

```text
shaped + - * /
x
Dot
Outer
Magnitude
Normalize
Cross
Contract
Reshape
ToShape
Permute
Transpose
BroadcastTo
```

Runtime shape validation returning `Err(ShapeError...)` is an ordinary explicit
error value and is not itself an effect.

The storage-producing operations:

```text
Create(request)
Materialize(request)
TransferTo(request)
Relayout(...)
```

must contribute the actual effects of the selected provider/helper path.
Depending on the concrete operation and target, these may include:

```text
MayAllocate
MayBlock
MaySuspend
MayIO
MayAccessVolatile
MayMutateExternalState
```

Do not introduce a public effect merely because the operation is called a
"tensor" or "memory transfer" operation. Preserve the existing effect lattice
and infer the concrete causes.

A synchronous `TransferTo` must not hide an outstanding asynchronous dependency.
An explicit asynchronous transfer contributes the corresponding suspension,
blocking, I/O, lifetime, and publication effects according to its actual
contract.
