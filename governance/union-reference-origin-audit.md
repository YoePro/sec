# Union constructor reference-origin audit

Owner: `frontend.union-constructor-value-semantics` in `declarations.yaml`.
Audited: 2026-11-21. Finding: `BUG-000036`.

## Contract and inspected paths

`rules/declarations/unions.md` §§5,7.8 uses the ordinary lifetime rules for
payloads. `rules/memory/lifetime_analysis.md` §§14,28 requires carried
references to outlive the returned aggregate and preserves caller-relative
returned relations; match-arm restrictions remain governed by §9 and
`rules/memory/ownership.md` §28. Reference storage is not the referent's storage.

The audit followed constructor selection/preparation, direct reference origins,
contained aggregate origins, local holder propagation, terminal escape checks,
returned-origin summaries and caller substitution. It inspected both payload
forms and the already shared ownership traversal. It did not reinterpret
ordinary functions with variant-like names as constructors.

## Finding and resolution

Before the fix, a single-payload constructor did not record its operand's
reference origin. `return Choice.Single(ref local)` was accepted, while
`return Choice.Named { value: ref local }` reported `S1128`. Bound/nested union
values and returned-origin summaries could consequently lose that dependency.
The initial two-case regression observed only the named-form error.

Accepted constructors now record reference contents from their typed operand
plans while lexical origins are available. Each variant and named payload field
has a distinct path. Direct references and nested aggregate reference contents
retain their canonical Places, local/match scope, ambiguity and unknown facts.
The outer aggregate does not acquire a referent Place of its own. A stable
path order selects the diagnostic representative without dropping dependencies.

Both constructor forms expose those recorded contents to aggregate propagation.
Queries do not rescan constructor argument/field AST lists or infer operands
again. Canonical returned-origin analysis and call substitution consume the
same contents; no separate union lifetime engine or exemption was introduced.
Ordinary factories that discard a borrowed argument retain no such dependency.

## Verified cases

- Shared and mutable local references cannot escape either payload form.
- A local dependency survives binding the union to another local.
- A reference nested in a struct payload remains a dependency.
- A union returned by a helper retains its actual caller's local dependency.
- A branch-scoped match payload cannot escape through either union form.
- Caller-provided shared/mutable references may be returned in either form.
- Local nonescaping use is accepted; a factory returning a payload-less variant
  does not invent retention of its borrowed argument.
- Stored constructor origins remain authoritative after AST operand-list removal;
  returned contained-origin maps are detached.
- All three Sema depths and both scalar plans run the paired fixtures. CLI check
  and IR diagnostics and unsaved LSP updates project the same rejection pairs
  and related referent locations; valid replacements clear diagnostics.

## Boundaries

This closes the constructor-origin audit and its verified omission. It does not
claim new reference/union runtime representation, native reference lowering,
full NLL/last-use precision, opaque foreign retention contracts, semantic-copy
lowering or unrelated match extraction capabilities. Those remain with their
existing memory, lifetime, FFI and backend owners. Unknown facts stay unknown;
scalar copyability is never used as proof that a carried reference may escape.
