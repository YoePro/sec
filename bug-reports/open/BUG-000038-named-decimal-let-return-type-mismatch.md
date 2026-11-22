# BUG-000038 — Returning a named-decimal local fails Semantic IR verification

- **Status:** open
- **Reported:** 2026-11-22 (found while integrating MD-049 fixed-scale facts)
- **Area:** Semantic IR — local storage and return of named decimal types
- **Code:** `internal/ir/semantic` (builder/verifier return-type check)
- **Rulebooks:** `rules/compiler/semantic_ir.md`, `rules/types/types.md`

## Reproduction

```sec
module main

type Money decimal

fn Literal() Money {
    let value: Money := 1.5
    return value
}
```

`sec emit-ir` reports:

```text
semantic IR verification/serialization error: return type mismatch
```

## Notes

- Reproduces without any MD-049 `scale` contract, so it predates the
  fixed-scale work; the same program with a fixed-scale `Money` fails the same way.
- Struct-field defaults and literals of named decimals (`Holder { money: 1.5 }`)
  verify correctly, so the mismatch is specific to the local binding/return path.
- Not fixed; triage should compare the stored local's type with the named
  result type at the return.
