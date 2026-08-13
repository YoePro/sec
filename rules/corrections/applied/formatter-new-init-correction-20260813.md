# Correction: formatter sync for `impl` revision 2.0

**Target:** `rules/tooling/formatter.md`  
**Source rule:** `rules/declarations/impl.md` revision 2.0  
**Date:** 2026-08-13

## Canonical forms

The formatter must preserve/write:

```sec
impl Type {
}
```

and:

```sec
impl extends Type {
}
```

for primary and extension implementation blocks.

## Implicit self

Canonical formatting must not introduce explicit receiver parameters such as:

```sec
ref self
ref mut self
```

Legacy/recovery syntax, when accepted by parser migration paths, should format
or diagnose toward implicit-self canonical source rather than preserving it as
preferred syntax.

## `init`

Format lifecycle initializers without `fn`:

```sec
init() {
}

init(size: uint) AllocationError {
}
```

The trailing type is formatted in the same type-reference style as other type
positions but must not be described as a return type by tooling.

## `free`

Format:

```sec
free {
}
```

as a special lifecycle member.

## `new`

Format lifecycle construction as:

```sec
new Type(args...)
try new Type(args...)
```

Do not rewrite:

```sec
Type(value)
```

into `new Type(value)` or the reverse. Conversion and lifecycle construction
have distinct semantics.

`new` must not be presented or formatted as an allocation operator implying
heap storage.

## Nested impl

Format nested implementations using ordinary impl block style:

```sec
impl Vehicle {
    type Engine struct {
        MaxPower: int,
    }

    impl Engine {
        fn Start() void {
        }
    }
}
```
