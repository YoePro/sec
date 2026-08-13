# Correction: lexical sync for lifecycle construction

**Target:** `rules/foundations/lexical_structure.md`  
**Source rule:** `rules/declarations/impl.md` revision 2.0  
**Date:** 2026-08-13

## `new` is a hard keyword

Add:

```text
new
```

to the general Sec hard keyword set.

This is a source-compatibility change. After implementation, `new` cannot be
used as a variable, function, type, field, property, parameter, module, import
alias, generic parameter, or other declaration name governed by hard-keyword
reservation.

Tooling inventories must be updated in the same change, as required by the
lexical rulebook's keyword-governance rule.

### Required compiler/tooling updates

- lexer token kind for `new`;
- keyword lookup table;
- parser expression-start classification;
- syntax highlighting;
- formatter token handling;
- LSP semantic-token classification;
- rename validation;
- diagnostics for declarations that use `new` as an identifier;
- tests for keyword reservation and `new Type(...)` tokenization.

## `init` remains contextual

Do **not** add `init` to the global hard keyword set for this change.

Inside an `impl` member position, bare:

```sec
init(...) {
}
```

or:

```sec
init(...) ErrorType {
}
```

introduces a lifecycle initializer.

Outside that contextual position, `init` remains subject to ordinary identifier
rules. For example, an ordinary function/member named `init` introduced with
`fn` is not the lifecycle `init` selected by `new`.

This keeps the language change to one new globally reserved keyword: `new`.
