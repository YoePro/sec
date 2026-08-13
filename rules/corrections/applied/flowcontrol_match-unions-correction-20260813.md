# Correction: Union match patterns, empty state, and shallow field destructuring

**Status:** Applied  
**Applied:** 2026-08-13

## Target

`rules/control-flow/flowcontrol_match.txt` or its canonical Markdown replacement.

## Required changes

### Remove the postponed status for union field destructuring

Direct field destructuring of struct-like union variant payloads is canonical and shallow.

Whole-payload binding remains valid:

```sec
Rectangle(rectangle) => Use(rectangle.width, rectangle.height)
```

Direct named-field binding is also valid:

```sec
Rectangle { width, height } => Use(width, height)
```

Partial binding is valid:

```sec
Rectangle { width } => Use(width)
```

Omitted pattern fields are simply not bound. They do not constrain the matched value and do not require `..`.

Rename is valid:

```sec
Rectangle {
    width: w
    height: h
} => Use(w, h)
```

Borrowed bindings follow normal ownership rules:

```sec
Rectangle {
    width: ref w
} => Use(w)
```

```sec
Rectangle {
    width: ref mut w
} => Modify(w)
```

Destructuring must not create hidden partial moves or hidden cloning.

Nested recursive patterns remain outside this rule. Use another `match` for nested variant/structured values.

### Add the compiler-known `empty` pattern

A union binding that may still be in its union-specific empty initialization state may be matched using:

```sec
match state {
    empty => Initialize()
    Idle => Wait()
    Running => Work()
}
```

`empty` is not a union variant.

If `empty` is reachable, exhaustiveness must cover it unless an unguarded catch-all covers the remaining states.

If the subject is proven initialized, `empty` is not part of exhaustiveness.

An unguarded `_` covers reachable `empty` state as well as remaining union variants.

Tooling may warn when a catch-all hides a reachable empty state without an explicit `empty` arm.

### Add relationship to `is`

`value is Variant` performs a non-destructuring active-variant test.

`value is empty` tests the compiler-known empty initialization state when that state is possible.

`is` does not bind payloads. Use `match` when payload extraction or exhaustive `Result`/`Option` handling is required.
