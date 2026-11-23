# BUG-000048 — Missing lambda return type drops the committed lambda

## Status

Fixed, 2026-11-23.

## Reproduction and rule

`testdata/parser/lambda_return_recovery/missing_invalid.sec` contains ordinary
and capture lambdas with parameter lists and bodies but no return type.
The original parseLambdaExpression reports the missing type and returns nil,
losing the callable/body instead of retaining the invalid signature.
Restoring the original parser in a temporary Go overlay makes the retention
regression fail.

`rules/compiler/parser_recovery.md` §§14.5,14.7,15,46.2 requires retaining
recoverable committed constructs with an invalid type rather than inferring
void or losing the node. Owner: `frontend.parser-long-term`.

## Resolution and limits

When the next token is a real body brace, preserve an InvalidTypeReference
with recovery metadata and parse the body normally. Parameters, captures,
sibling statements and later declarations remain in the AST. Sema gives the
invalid signature no callable identity and still diagnoses independent later
errors. CLI/LSP retain the two primary syntax diagnostics without dependent
cascades, and no invalid IR artifact is written.

Missing-body lambda recovery and the broader remaining specialized type
positions are outside this fix. The broader governance requirement remains
partial, not complete.
