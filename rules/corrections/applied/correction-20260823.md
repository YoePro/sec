# Correction: CR line endings are not honored by newline-sensitive lexer tokens

**Repository baseline:** `main` @ `c515862`
**Rulebook:** `rules/foundations/lexical_structure.md`
**Implementation:** `internal/lexer/lexer.go`

## Defect

The lexical rulebook defines LF (`\n`), CRLF (`\r\n`), and CR (`\r`) as accepted source line endings and requires all three to behave as line breaks.

The current lexer does not apply that rule consistently to newline-sensitive token readers:

- `readLineComment()` terminates only on `\n`.
- `readCharLiteral()` rejects a physical newline only when it encounters `\n`.
- `readStringBody()` rejects a physical newline only when it encounters `\n`.

This is more than the already documented incomplete CR/CRLF **line accounting**. It changes token boundaries and literal validity.

## Incorrect behavior

### Bare CR after a line comment

For source equivalent to:

```text
// comment\rlet value := 1
```

`\r` is a normative line ending. The comment must end before it. The current lexer continues the comment until LF or EOF and may therefore absorb the following Sec source into the comment token.

### CRLF after a line comment

For:

```text
// comment\r\nlet value := 1
```

The line ending must not be part of the comment token. The current lexer stops at LF, so the preceding CR is included in the comment lexeme.

### Bare CR inside ordinary string or character literal

A physical CR is a newline under the normative rule. It must therefore terminate an ordinary string/character literal as an error just as LF does. The current readers can consume it as literal content.

## Required correction

Introduce one canonical lexer notion of a physical line ending and use it consistently for:

1. line and column advancement;
2. line-comment termination;
3. ordinary-string physical-newline rejection;
4. character-literal physical-newline rejection;
5. any other lexer path whose semantics depend on source line boundaries.

CRLF must be treated as one line break, not two. A line-ending sequence must not become part of a line-comment token.

The implementation may normalize line endings before tokenization or handle LF/CRLF/CR explicitly, provided exact source mapping and required byte offsets remain correct.

## Required regression tests

Add lexer tests covering at least:

- line comment followed by LF;
- line comment followed by CRLF;
- line comment followed by bare CR;
- token immediately following each of those comments;
- ordinary string containing unescaped LF, CRLF, and bare CR;
- character literal containing unescaped LF, CRLF, and bare CR;
- line/column behavior after LF, CRLF, and CR;
- comment lexeme exclusion of the complete line ending.

## Governance impact

Keep the existing `CR and CRLF line accounting` partial-status item, but do not treat it as sufficient documentation of this defect. The semantic token-boundary/literal-validity defect should remain tracked until the correction and regression tests are implemented.

## Applied

Applied on 2026-08-23 to:

- `internal/lexer/lexer.go`;
- `internal/lexer/lexer_test.go`;
- `implementation-status.yaml`.

The lexer now uses one physical-line boundary for LF, CRLF, and bare CR in
source decoding, token position advancement, line-comment termination, and
ordinary string/character literal rejection. CRLF is consumed as one physical
line ending and is excluded completely from line-comment tokens.

Verified with:

```text
go test ./internal/lexer -count=1
```
