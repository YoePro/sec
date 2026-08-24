# Diagnostics correction — mentor-style error and ownership diagnostics

- **Status:** Applied normative correction
- **Applied:** 2026-08-24
- **Created:** 2026-08-24
- **Last updated:** 2026-08-24
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `b3315f6` (semantic parent `45e5cd4`)
- **Target rulebook:** `rules/tooling/diagnostics.txt`

---

## Correction

Error-handling diagnostics must follow the Sec compiler mentor principle.
The primary message must be understandable without compiler-theory vocabulary.

### Required diagnostic structure

Where relevant, identify:

1. what source value or operation is involved;
2. what earlier source operation caused the current state;
3. why the current operation is invalid;
4. the relevant expected/actual error or success type;
5. a practical source-level fix when one is known.

### Consumed Result example

For:

```sec
let result := Read()
let value := result.Ok()
Use(result)
```

prefer a diagnostic shape such as:

```text
error: `result` cannot be used here because `result.Ok()` consumed it

`result.Ok()` keeps the success side as an Option and consumes the original Result.
The later use therefore has no value left to read.

help: use `result` before calling `.Ok()`, borrow with `OkRef`, or restructure the code
```

Do not require the programmer to understand terms such as "affine projection"
or "ownership lattice" to understand the primary message.

### Required focused diagnostics

Provide focused diagnostics for at least:

```text
non-error type used as Result E
invalid concrete-to-concrete error propagation
invalid error narrowing
invalid Result/Option pattern family
Ok handler inside try
Some handler inside try
obsolete try { match { ... } } wrapper
unreachable try handler and covering earlier handler
unmatched partial-try failure incompatible with enclosing channel
heterogeneous Err(binding) with no single declared binding type
missing try on fallible assignment
missing explicit error type on try set
return Ok() inside try set
fallible getter syntax in Sec 0.1
use after consuming Result.Ok()/Err()
non-discardable payload abandoned by consuming Result projection
invalid `is not Some(binding)`
```

Diagnostics and LSP must use the same Sema ownership/error facts.

## Cross-reference

```text
rules/errors/errorhandling.md
rules/tooling/lsp.md
```
