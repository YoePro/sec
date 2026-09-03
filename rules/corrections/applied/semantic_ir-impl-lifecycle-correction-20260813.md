# Correction: Semantic IR lifecycle construction

**Target:** `rules/compiler/semantic_ir.md`
**Source rule:** `rules/declarations/impl.md` revision 2.0  
**Date:** 2026-08-13

## Construction resolution

The source distinction:

```sec
Type(value)
new Type(args...)
```

must be resolved before Semantic IR generation.

Semantic IR must preserve different semantics for:

- explicit conversion/casting;
- ordinary aggregate/default construction;
- lifecycle construction selected by `new`.

No unresolved source-level `new` token is required after semantic resolution,
but the resolved construction operation must retain its lifecycle meaning.

## Lifecycle construction operation

A resolved lifecycle construction operation must identify at least:

- target type;
- selected explicit `init` overload or permitted implicit construction path;
- argument values and ownership actions;
- construction error type when fallible;
- initialization state/partial-initialization tracking needed for cleanup;
- source location;
- resource/effect metadata required by later analysis/lowering.

A successful lifecycle construction operation produces exactly one value of the
target type.

A fallible initializer does not produce a separate Result success payload merely
because source uses `try new`. Semantic IR may use an explicit success/failure
branch or another canonical checked-construction representation, but it must
preserve:

```text
success -> constructed Type value
failure -> exact initializer error value
```

## Failure cleanup

The failure edge must execute or reference cleanup for successfully initialized
fields/resources and construction temporaries.

It must not invoke the completed-value custom `free` operation for an instance
that never completed construction.

The success edge transfers the fully initialized instance into ordinary
ownership/destruction semantics.

## `free`

Custom `free` must resolve to explicit destruction behavior associated with the
concrete target type.

It must remain distinguishable from an ordinary callable method and must not be
represented as a user-callable `value.free()` member.

## No implicit heap semantics

Lifecycle construction must not lower to heap allocation solely because the
source used `new`.

Storage placement/allocation remains explicit or derivable from the construction
body, storage rules, escape analysis, allocation authority, and optimization.

Semantic IR should retain enough information to permit allocation elision and
non-heap placement when semantics allow it.

## Verification

Extend Semantic IR verification so that:

- every successful lifecycle construction result is fully initialized;
- every failure path cleans initialized construction state exactly once;
- no incomplete instance escapes;
- exact construction error typing is preserved;
- no required cleanup is lost;
- `new` introduces no implicit allocation operation without semantic cause.
