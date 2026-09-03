# Semantic IR correction — spread semantics

## Metadata

- **Status:** Applied 2026-08-13
- **Created:** 2026-08-13
- **Last updated:** 2026-08-13
- **Target:** `rules/compiler/semantic_ir.md`
- **Canonical source:** `rules/declarations/spread.md`

## Required semantic contract

Semantic IR must preserve spread semantics until they are safely normalized.
The representation must make the following facts explicit or otherwise proven:

- spread source evaluation occurs exactly once;
- destination context is known;
- fixed expansion count is known where required;
- array expansion order is increasing index order;
- struct expansion order is declaration order;
- call expansion occurs before final parameter/overload matching;
- every expanded use has a resolved copy/borrow/rejection ownership operation;
- struct spread override resolution is left-to-right;
- omitted-field defaults are applied only after spread/override resolution;
- no hidden allocation is introduced by spread.

A dedicated operation family such as the following is permitted but not
required:

```text
SpreadCallArguments
SpreadArrayElements
SpreadStructFields
```

An implementation may instead normalize spread to ordinary call arguments,
array elements, and field initializers before the final Semantic IR form, but
only after all semantic facts above have been resolved and preserved.

MLIR/LLVM lowering must not infer ownership, ordering, or struct override
semantics from source syntax.
