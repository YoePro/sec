# Grammar correction — spread rulebook synchronization

## Metadata

- **Status:** Applied 2026-08-13
- **Created:** 2026-08-13
- **Last updated:** 2026-08-13
- **Target:** `rules/foundations/grammar.md`
- **Canonical source:** `rules/declarations/spread.md`

## Required changes

1. Replace references to `spread.txt` with `spread.md`.
2. Keep postfix spread grammar:

```text
SpreadExpression
    ::= Expression "..."
```

3. Keep canonical spread-aware contexts:

```text
CallArgument
ArrayElement
StructLiteralEntry
```

4. Clarify that runtime-length sequence spread is invalid for fixed-arity calls
   and fixed-array literals. If a future/current function rule defines a
   runtime-arity destination, that destination may accept runtime-length spread
   according to its own parameter semantics.
5. Do not add a consuming-spread grammar form. `expression...` is not implicit
   move syntax.
6. Ensure grammar references the canonical struct rule that omitted Defaultable
   fields are completed only after spread and explicit override resolution.

## No syntax change

This correction does not introduce a new token or keyword. The canonical postfix
`...` spelling remains unchanged.
