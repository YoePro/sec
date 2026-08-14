# Correction: generic impl and method-level generics

- **Status:** Applied 2026-08-14
- **Created:** 2026-08-14
- **Last updated:** 2026-08-14
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Target:** `rules/declarations/impl.md`

## Generic primary implementation

A generic nominal type uses its declared parameters in the primary impl target.

```sec
type Box[T] struct {
    value: T,
}

impl Box[T] {
    fn Value() T {
        return self.value
    }
}
```

The target parameters correspond to the owning generic type parameters.

They are not unrelated implicit type declarations.

## Generic extension fragments

Same-module extension fragments are permitted for generic types.

```sec
impl extends Box[T] {
    fn IsSet() bool {
        ...
    }
}
```

Primary and extension fragments compose into one concrete member surface for every monomorphized `Box[T]`.

## Method-level generics

Methods may introduce additional generic parameters.

```sec
impl Box[T] {
    fn Transform[U](transform: fn(T) U) Box[U] {
        ...
    }
}
```

Method-level generics are canonical Sec 0.1 behavior and must not be documented as postponed.

Concrete impl methods continue to use implicit `self`.
