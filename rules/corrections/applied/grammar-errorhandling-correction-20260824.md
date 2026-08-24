# Grammar correction — error handling revision 2

- **Status:** Applied normative correction
- **Applied:** 2026-08-24
- **Created:** 2026-08-24
- **Last updated:** 2026-08-24
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `b3315f6` (semantic parent `45e5cd4`)
- **Target rulebook:** `rules/foundations/grammar.md`

---

## Correction

Synchronize the canonical grammar with `rules/errors/errorhandling.md` revision 2.0.

### Error-marked declarations

Add the error marker to canonical enum and tagged-union declaration grammar.

Conceptually:

```text
EnumDeclaration
    ::= "enum" Identifier [ EnumUnderlyingType ] [ "error" ] EnumBody

UnionDeclaration
    ::= "type" Identifier "union" [ "error" ] UnionBody
```

When present, `error` is the final semantic marker before the declaration body.
It is not an enum underlying type and does not introduce general inheritance.

Canonical examples:

```sec
enum IOError error {
    ReadError
}
```

```sec
enum ProtocolError uint16 error {
    InvalidFrame = 1
}
```

```sec
type DetailedError union error {
    Failed {
        Code: int
    }
}
```

### Fallible setter declaration

Replace canonical:

```sec
try set value {
    ...
}
```

with:

```sec
try set value ErrorType {
    ...
}
```

Conceptually:

```text
FallibleSetter
    ::= "try" "set" Identifier TypeReference AccessorBody
```

Interface requirements use the same explicit error type without a body.

### `try` operand surface

`try` remains an expression prefix and is not restricted by grammar to a
`Result`-typed call. Sema determines whether the protected expression is a
language-defined fallible/short-circuit expression.

The parser must therefore accept `try` in ordinary expression positions such as:

```sec
Process(try Read())
```

```sec
if try IsReady() {
    Proceed()
}
```

```sec
let item := Container {
    Value: try ReadValue()
}
```

Assignment remains a statement and does not become an expression.

### Try handlers

Canonical handler syntax is:

```text
TryHandlerBlock
    ::= "{" TryHandler+ "}"

TryHandler
    ::= TryPattern [ "where" Expression ] "=>" HandlerBody
```

For Result/error `try`, only `Err(...)` patterns are valid.
For Option `try`, only `None` is valid.
The exact permitted pattern family is resolved by Sema from the protected type.

Remove canonical support for:

```text
Ok(...) handlers in try
Some(...) handlers in try
try { match { ... } }
```

The obsolete nested `match` wrapper may be recognized only for a focused
migration diagnostic; it is not valid Sec 0.1 source.

### Partial handler lists

The grammar must not encode try-handler exhaustiveness.
Try handler lists are partial by semantic rule; unmatched failure/absence states
propagate when compatible.

### `return try`

No dedicated lexical token is needed. Ordinary return-expression grammar must
accept a `TryExpression`:

```sec
return try Load()
```

Sema applies the Result return-forwarding rule.

### Option payload binding in `if`

The grammar/parser must preserve enough `is` syntax to recognize the narrow
positive binding form:

```sec
if option is Some(value) {
    ...
}
```

This does not introduce general pattern-binding `if` grammar. Sema restricts the
exception to the canonical Option `Some(binding)` positive-`is` form.

## Cross-reference

Canonical semantics are defined by:

```text
rules/errors/errorhandling.md
rules/control-flow/flowcontrol_if.md
rules/control-flow/flowcontrol_match.md
rules/declarations/properties.md
```
