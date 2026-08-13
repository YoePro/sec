# LSP correction — enum domain and checked conversion visibility

**Status:** Applied
**Applied:** 2026-08-13
**Language version:** Sec 0.1
**Created:** 2026-08-13
**Last updated:** 2026-08-13
**Target:** `rules/tooling/lsp.md`
**Source of truth:** `rules/declarations/enums.md`

Enum hover information should expose the semantic domain when it matters for correct code:

```text
ordinary enum
    closed; only declared numeric value classes are valid

bit-backed enum
    open; every value representable by bit[N] is valid
```

For a bit-backed hardware enum, hover should make clear that undeclared bit patterns may occur
at runtime.

When a `match` lists declared members but leaves undeclared bit patterns uncovered, the LSP
should surface the non-exhaustiveness clearly and may suggest a final `_` fallback.

For a checked runtime integer-to-enum conversion, hover/signature information should show the
fallibility and `EnumValueError` family so the programmer can see why `try` is required.
