# Correction: `->` punctuation for consuming interface methods

- **Status:** Applied 2026-08-13
- **Created:** 2026-08-13
- **Last updated:** 2026-08-13
- **Document revision:** 1
- **Sec language version:** 0.1

Add `->` as a punctuation token where needed for consuming interface method declarations.

`->` is not a keyword.

Canonical use:

```sec
interface Resource {
    -> fn Detach() Handle
}
```

In this position, `->` denotes a consuming receiver contract. It does not denote a return type.

This addition must be reflected in lexer/token definitions, parser expression/declaration handling as applicable, formatter, syntax highlighting, and LSP tokenization.
