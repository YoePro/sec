# BUG-000032 — `<` comparison read as unit/type arguments when a later `> (` follows

- **Status:** open
- **Reported:** 2026-11-20 (found while writing `sec/stdlib/net/http/http1.sec`)
- **Area:** parser — expressions versus unit/type-argument lists
- **Code:** `internal/parser/parser.go` (unit-expression scanning near the
  "invalid unit expression" error, around line 6028)
- **Rulebooks:** `rules/foundations/grammar.md`, `rules/foundations/operators.md`,
  `rules/types/units.md`

## Reproduction

```sec
module main

fn F(a: int, b: int, c: int) bool {
    if a < b {
        return c > (a)
    }
    return false
}
```

`sec parse` reports:

```text
parse error: invalid unit expression b{returnc: unexpected "{"
parse error: expected '{' after if condition at 6:5
parse error: unexpected '}' ...
```

## Observations

- The parser treats `< b { return c >` as a unit/type-argument list because a
  `>` followed by `(` appears later, even across the `{` that ends the
  condition. The block brace should end any speculative argument list.
- A later `>` without a following `(` does not trigger it (`if total > 10`).
- Writing the comparison the other way (`if b > a {`) parses correctly; that
  is the workaround used in http1.sec (`while last > index`, `199 >= code`).
- In http1.sec the trigger was `while index < last { ... if number >
  (uint64.Max - addend) / 10 { ...` and `code < 200 || ... { ... }` followed
  by later parenthesized `>` comparisons.

## Expected

`a < b` is an ordinary comparison. Unit/type-argument scanning must not cross
a `{`, `}` or statement boundary, and an invalid speculative list must fall
back to expression parsing instead of reporting an error.
