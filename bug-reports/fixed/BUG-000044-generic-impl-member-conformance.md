# BUG-000044 — Generic impl conformance loses member and setter substitutions

## Status

Fixed on 2026-11-23.

## Report

Valid generic implementations containing properties and events failed concrete
interface constraints. Fallible setters using an owning `E: error` parameter
also reported `unknown type E`: deferred interface resolution and impl setter
resolution ran outside the declaration's generic scope.

## Reproduction

Source fixtures: `testdata/sema/generic_impl_conformance/valid.sec` and
`setters.sec`. Before the fix the Sema regressions reported:

- `type Box[int] does not satisfy constraint Contract[int] for X`.
- `unknown type E` at the interface and implementation setter contracts.

Run `go test ./internal/sema -run 'TestGenericImpl.*Conformance$' -count=1`.

## Expected and actual behavior

The complete explicit interface contract must substitute the owner's arguments
consistently. Valid member types and generic setter error contracts are now
accepted. Inherited setters resolve the declaring parent's arguments even when
the child reorders them. Concrete int/bool instances leave the template intact.

The negative fixture still rejects four incompatible members, four missing
members, one unrelated setter error and structural-only conformance. The nine
interface member errors retain S1106/S1107 identities and requirement locations
through compiler JSON and LSP. The separate generic-constraint rejection keeps
its existing unregistered diagnostic; this report does not claim to fix that
registry gap.

## Scope

- Area: generic impl interface conformance in Sema and its public frontend projections.
- Rulebooks: `rules/declarations/generics.md` §§8,14,15A,22,25,30;
  `rules/declarations/interfaces.md` §§4–6,9,12;
  `rules/errors/errorhandling.md` §24.1.
- Governance integration: `frontend.generics-v2`.
- Affected phase: semantic analysis, compiler diagnostics, LSP diagnostics.

## Resolution

Share detached property/event substitution between generic type instantiation
and forwarded types. Preserve the declaration environment for deferred setter
error resolution; resolve implementation setter errors inside the impl scope.
Resolve inherited error contracts through the declaring interface instance and
clear deferred records between analyses. Extract instantiation into a focused
Sema file, retaining the existing nominal caching behavior.

Validation: full `internal/sema`, `internal/ir/semantic`, `internal/traceability`
suites pass; focused compiler/LSP conformance and existing interface/setter tests
pass. Source-backed regressions cover both acceptance and rejection, template
isolation, inherited argument order and reanalysis. This fixes frontend
conformance; runtime/backend support remains tracked separately.
