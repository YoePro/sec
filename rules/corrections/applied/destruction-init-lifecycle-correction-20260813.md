# Correction: destruction sync for `init` / `free` lifecycle

**Target:** `rules/memory/destruction.txt`  
**Source rule:** `rules/declarations/impl.md` revision 2.0  
**Date:** 2026-08-13

## Preserve existing destruction model

Keep the existing rules that:

- compiler-derived destruction is preferred;
- custom `free` is needed only for resources not released through owned-field
  destruction;
- every explicit allocation has a defined ownership/deallocation strategy;
- partial construction destroys only successfully initialized state;
- custom `free` runs only for fully constructed values;
- direct user calls such as `value.free()` are prohibited.

These existing rules become the destruction half of the explicit lifecycle model
in `impl.md` revision 2.0.

## Add explicit `init` integration

Construction through:

```sec
new Type(args...)
```

selects a lifecycle `init` member or a permitted implicit construction path.

An `init` body may establish fields/resources incrementally.

On successful completion:

- the result is one fully valid instance of the impl target;
- normal lifetime/destruction responsibility begins.

On fallible construction failure:

- no completed outer instance escapes;
- only successfully initialized owned fields/resources are cleaned up;
- cleanup follows reverse successful initialization/order rules where relevant;
- the completed-value custom `free` block is not invoked;
- explicitly tracked construction temporaries are cleaned up according to their
  ownership semantics.

## Construction/destruction balance analysis

Add the normative analysis rule:

> Every resource acquired during `init` must have a statically defined cleanup
> path both for every failure path after acquisition and for eventual
> destruction of a successfully constructed value.

A handwritten `free` block is **not** mandatory when owned fields already encode
and perform the required cleanup.

Custom `free` is required only when no existing owned-field or other explicit
ownership mechanism supplies the required release.

The compiler/analysis pipeline should diagnose when it can prove:

- resource acquisition with no cleanup path;
- incompatible allocator/deallocator pairing;
- duplicate release;
- cleanup missing on one construction error path;
- partial construction escaping;
- completed-value `free` being applied to incomplete construction.

## `new` is not allocation

Explicitly state that the `new` syntax itself does not imply heap allocation.

Stack, inline, register/SSA, arena, or other valid storage placement remains an
independent storage decision. Destruction semantics apply to the constructed
value regardless of storage placement.

## References

Add `rules/declarations/impl.md` revision 2.0 as the owner of source-level
`init`, `free`, and `new` lifecycle syntax.
