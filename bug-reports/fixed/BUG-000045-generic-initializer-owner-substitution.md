# BUG-000045 — Generic initializer ignores constructed owner arguments

## Status

Fixed, 2026-11-23.

## Reproduction

`testdata/sema/generic_impl_composition/primary.sec` and `extension.sec`
declare `Box[T]` and a same-module extension with `init(value: T)`.
Before the fix, `new Box[int](3)` reported
`argument 1 to @init:Box must be T, got int`.

## Expected behavior and scope

The explicitly constructed `Box[int]` fixes the owner's `T` before checking
initializer arguments. Primary and extension declarations compose into the
same concrete member surface. Fallible initialization must likewise retain
the substituted construction target and error channel.

Rules: `rules/declarations/impl.md` §§15–16,21–22;
`rules/declarations/generics.md` §§8–9,17,25,29;
`rules/compiler/monomorphization.md` §§4,8–9.
Governance: `frontend.generics-v2`. Affected phase: Sema and editor metadata.

## Resolution

Bind owner arguments on detached init candidates before the existing call
pipeline checks overloads, contextual arguments, ownership and effects. The
initializer's construction type, error type and specialization identity retain
the same substitution. Template declarations are not replaced or mutated.

Regression tests cover int/bool instances, both fragment orders, overloads,
forwarded generics, contextual None, two error channels, concrete signature
previews, canonical demand identities and invalid arguments through CLI/LSP.
Native lifecycle construction lowering remains outside this fix.
