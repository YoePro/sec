# Correction: callable types and lambdas in functions v2

- **Status:** Applied 2026-08-14
- **Created:** 2026-08-14
- **Last updated:** 2026-08-14
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Target:** `rules/declarations/functions.md`

Function/callable types preserve callable environment capability in addition to parameter contracts.

Examples:

```sec
fn(int) int
mut fn() int
-> fn() Resource
fn(-> Resource) void
mut fn(ref mut Buffer) void
```

Meanings:

- `fn`: shared/reusable callable value;
- `mut fn`: invoking requires mutable/exclusive callable access;
- `-> fn`: invoking consumes the callable value.

Lambda expressions are still written with plain `fn`; their callable capability is inferred from the body/captures.

Callable type assignability must not erase a stronger source requirement.

A callable requiring less authority may be placed behind a stricter target contract, but never the reverse.
