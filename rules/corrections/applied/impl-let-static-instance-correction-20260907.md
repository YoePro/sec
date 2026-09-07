# Impl let static/instance correction

Applied 2026-09-07 from the user's explicit correction: inside `impl`,
`static let Var: T := v` and `let Var: T := v` are not equivalent.

- Explicit `static` makes a member type-owned and available without an instance.
- Without `static`, the member belongs to an instance and requires that instance.
- Mutability does not erase this distinction.
- Instance initialization must not become static initialization.
- Formatting must preserve `static` in an impl binding; S1026's redundant-static
  advice is withdrawn.

The correction is integrated in `rules/declarations/static.md`,
`rules/declarations/impl.md`, `rules/foundations/grammar.md`,
`rules/foundations/names_scopes_visibility.md`, `rules/tooling/formatter.md`,
and `rules/tooling/lsp.md`.

Implementation progress belongs to `frontend.static-declarations-members` in
`implementation-status.yaml`. In particular, correcting the classification does
not establish per-instance storage support: that missing implementation must be
diagnosed rather than silently approximated with static storage.
