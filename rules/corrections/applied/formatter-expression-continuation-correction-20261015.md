# Configured indentation of return and if expression continuations

- **Status:** Applied 2026-10-15
- **Decision authority:** Language owner, 2026-10-15
- **Canonical rulebook:** `rules/tooling/formatter.md`, revision 2.1
- **Governance:** `governance/formatting.yaml`, `tooling.formatter-v2`

The owner clarified that an expression beginning beside `return` keeps its
first operand there and indents subsequent continuation lines by one configured
indentation width. The default is four spaces; widths two and six remain valid.
This layout preserves existing operator placement instead of applying the
generic leading-operator rewrite to inline-start return expressions.

The owner also requested the same correction for multiline `if` and `else if`
conditions. Each continuation uses one configured step relative to the header
line's structural indentation; the statement body retains its ordinary level.

The clarification is applied to § 6(3) and the exception in § 9(15). Nested
delimiter layouts, comments, multiline literal contents and subsequent
statements retain their own structure. No expression, condition or return
semantics change.
