# Correction: names/scopes sync for `impl` revision 2.0

**Target:** `rules/foundations/names_scopes_visibility.md`  
**Source rule:** `rules/declarations/impl.md` revision 2.0  
**Date:** 2026-08-13

## Combined implementation member scope

Retain and strengthen the existing rule that the primary `impl Type` and every
valid same-module `impl extends Type` contribute to one combined member
namespace.

Duplicate/conflict checks must cover all source files in the module.

## `init` overload group

Add `init` as a special lifecycle overload group in the type's implementation
surface.

`init` overload identity uses parameter signature only.

The optional trailing construction error type does not participate in identity
or overload resolution.

Invalid:

```sec
impl Resource {
    init(path: string) IOError {
    }

    init(path: string) ParseError {
    }
}
```

## `free`

`free` is a special non-overloadable lifecycle member. At most one `free` may
exist across the complete primary+extension implementation surface.

It must not collide with another member category in a way that makes source
lookup ambiguous.

## Associated declarations

Keep nested/associated declarations in the target type's member namespace.
Their stored representation belongs to the nested type, not to the outer target.

## Nested implementation ownership

A nested `impl NestedType` inside `impl Owner` is valid only when `NestedType` is
owned by `Owner`.

It must resolve semantically as the primary implementation of the qualified
nested type, for example:

```text
Vehicle.Engine
```

and must not become a mechanism for grouping an unrelated module-level type's
implementation.

## Defining-module protection

Add the coherence rule:

> User-defined ordinary impls may be declared only in the module that defines
> the target type.

The existing compiler/core privilege for approved compiler-known types is an
explicit exception, not a general extension-method facility.

## Keyword reservation

Because `new` becomes a hard keyword, reserved-name and rename checks must reject
`new` as a declaration identifier after the lexical change is implemented.

Update stale references from `impl.txt` to `impl.md` revision 2.0.
