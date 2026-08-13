# Correction: Union defaults and empty initialization state

**Status:** Applied  
**Applied:** 2026-08-13

## Target

`rules/types/default_values.md`

## Required changes

Replace the current placeholder union-default section with the following semantics.

### Union defaultability

A union has no implicit first-variant default.

A union is `Defaultable` only when exactly one declared variant is explicitly marked `default` and that variant can be default-constructed.

Payload rules:

- a payload-less default variant is constructible;
- a single-payload default variant requires a Defaultable payload type;
- a struct-like default variant uses the normal omitted-field/default rules and is valid only when all required payload state can be constructed.

Example:

```sec
type State union {
    Idle default
    Running
}
```

For a mutable declaration:

```sec
let mut state: State
```

the resolved default is `State.Idle`.

### Union-specific empty mutable-declaration exception

Add one explicit exception to the general rule that a mutable declaration may omit an initializer only for a Defaultable type.

A mutable binding of a NonDefaultable union may omit its initializer:

```sec
type State union {
    Idle
    Running
}

let mut state: State
```

The binding begins in the compiler-known `empty` initialization state.

`empty` is not a default value. It is not a valid semantic union value, not `null`, not `None`, not `Nothing`, and not a hidden union variant.

The binding must contain a real union value before it may escape or be used in any context requiring a value. `is empty` and the `empty` match pattern may inspect this initialization state according to the union and match rulebooks.

Immutable bindings still always require an explicit initializer.

### Omitted struct-like union payload fields

A struct-like union payload field may be omitted when its type is Defaultable. The omitted field receives that type's normal resolved default.

A NonDefaultable payload field must be supplied explicitly.
