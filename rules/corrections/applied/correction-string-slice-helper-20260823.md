# Correction — Internal core helpers may contain direct MLIR/LLVM

**Date:** 2026-08-18
**Scope:** `core/string.sec`, internal `__...` helpers, compiler-known vs. privileged core implementation

## Correction

A previous recommendation treated `__StringSliceUnchecked(self, start, end)` as if it necessarily had to become a new compiler-known function with dedicated Sema registration and a special backend emission path.

That is too restrictive.

Sec core may use small internal `__...` helper functions whose implementation is written directly in the supported low-level MLIR/LLVM form. Therefore a representation-sensitive helper does **not** automatically require a new public or compiler-known source-level function.

The correct separation is:

```text
public/core semantic operation
    may be implemented in ordinary Sec

internal representation-sensitive helper
    may be implemented as a small privileged core __... function
    whose body contains direct MLIR/LLVM when appropriate

compiler-known member/intrinsic
    is required only when the compiler must own the semantic identity,
    resolution, analysis facts, or lowering contract independently of core source
```

## Consequence for string slicing

The public API remains ordinary core code:

```sec
fn Slice(start: uint, end: uint) Result[string, RangeError] {
    // validate start/end
    // then call the internal unchecked helper
}
```

The low-level operation may live in core as:

```sec
fn __StringSliceUnchecked(
    value: string,
    start: uint,
    end: uint,
) string {
    // direct MLIR/LLVM implementation
}
```

The exact low-level body must use the established Sec inline MLIR/LLVM syntax and contracts.

Its intended operation is representation-level only:

```text
result.data = value.data + start
result.len  = end - start
```

It performs:

- no bounds checking;
- no allocation;
- no string-content validation beyond the invariants already established by its caller;
- no user-visible error handling.

The checked `Slice` wrapper owns those source-level checks.

## Why this is preferable here

`__StringSliceUnchecked` is an implementation detail of core string behavior.

Making it a new compiler-known global function merely because it needs representation-level code would:

- enlarge the compiler-known registry unnecessarily;
- add Sema/backend special cases that core can own;
- move implementation detail out of `core/string.sec`;
- weaken dogfooding by bypassing the privileged-core mechanism.

The compiler-known rule remains important: compiler-known describes **semantic authority**, not a requirement that every physical implementation be hard-coded in the compiler. A compiler-known operation may use a core helper, privileged core implementation, Sec MLIR operation, target helper, or inline lowering.

## When compiler-known is still appropriate

A function/member should still be compiler-known when the compiler must know its semantics independently of the core implementation, for example because it affects:

- canonical member availability or name resolution;
- ownership or borrowing;
- effects;
- unsafe obligations;
- allocation semantics;
- target/layout facts;
- Semantic IR operation identity;
- mandatory optimization-independent semantics.

An internal `__...` helper does not become compiler-known solely because its body contains MLIR/LLVM.

## String implementation direction

For the current `core/string.sec` work:

```text
Slice
    -> checked ordinary core wrapper
    -> __StringSliceUnchecked
       -> small direct MLIR/LLVM helper

IndexOf / StartsWith / EndsWith / Contains
    -> ordinary Sec algorithms where practical

Split
    -> ordinary Sec state/algorithm
    -> uses the string primitives above
```

This keeps low-level representation knowledge narrow while allowing most string behavior to remain dogfooded Sec code.

## Additional implementation note

Inline low-level code remains subject to its declared safety/effect contract. Direct MLIR/LLVM in a core helper is not a mechanism for bypassing Sec ownership, lifetime, effect, or unsafe semantics.

## Superseded recommendation

The earlier recommendation to implement `__StringSliceUnchecked` by default as:

```text
new CompilerKnownFunction registry entry
    +
special emitCallExpression case
    +
dedicated direct-LLVM generator function
```

is superseded.

That route remains available when semantic/compiler ownership requires it, but it is **not the default requirement** for a small privileged internal core helper.

## Applied

Applied on 2026-08-23 to:

- `rules/compiler/compiler_known_members.md`;
- `rules/library/core-library.md`;
- `rules/types/types.md`;
- `implementation-status.yaml`.

The canonical rulebooks now assign the unchecked string-slice implementation
to a private privileged-core helper by default. The existing compiler-known
compatibility path remains recorded as partial implementation debt until the
direct MLIR/LLVM core-body syntax referenced by this correction is defined and
implemented.
