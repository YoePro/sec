# Correction: interface examples and impl-v2 boundary

**Target:** `rules/declarations/interfaces.txt`  
**Source rule:** `rules/declarations/impl.md` revision 2.0  
**Date:** 2026-08-13  
**Scope:** narrow synchronization only; do not settle the dedicated interface-implementation syntax in this correction

## Implicit self

Replace canonical interface/method examples that explicitly write:

```sec
ref self
ref mut self
```

with the current implicit-self method model.

For example:

```sec
interface Vehicle {
    fn Start() void
    fn Stop() void
}
```

and:

```sec
impl Car {
    fn Start() void {
        // ...
    }

    fn Stop() void {
        // ...
    }
}
```

Receiver mutation/borrowing requirements are compiler-derived/validated and are
not written as receiver parameters in canonical source.

## Ordinary impl boundary

Reference `rules/declarations/impl.md` revision 2.0 for:

- exactly one primary ordinary impl in the target type's defining module;
- optional same-module `impl extends Type` fragments;
- one merged ordinary member surface;
- implicit self;
- stored-representation separation.

Interface conformance must not create a second ordinary primary implementation
or permit another module to extend the ordinary member surface of an imported
type.

## Dedicated interface syntax remains owned here

Do not use this correction to decide whether final interface conformance syntax
remains a type-declaration `implements` form, becomes an interface-specific impl
form, or combines those mechanisms.

That decision belongs to the dedicated `interfaces.txt -> interfaces.md` review.

Until that review, preserve the current interface-specific syntax/status except
where it directly conflicts with canonical implicit-self or primary/extension
ordinary-impl rules.
