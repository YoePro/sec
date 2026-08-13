# Grammar correction — enum defaults and Go-style initializer repetition

**Status:** Applied
**Applied:** 2026-08-13
**Language version:** Sec 0.1
**Created:** 2026-08-13
**Last updated:** 2026-08-13
**Target:** `rules/foundations/grammar.md`
**Source of truth:** `rules/declarations/enums.md`

## Required correction

Replace the enum member production:

```text
EnumValue
    ::= Identifier [ "=" ConstantExpression ]
```

with:

```text
EnumValue
    ::= Identifier [ "default" ] [ "=" ConstantExpression ]
```

The order is normative:

```sec
MEMBER default
MEMBER default = expression
```

`default MEMBER` is not enum-member syntax.

Keep the existing rule that an enum must contain at least one member.

## Semantic note for the grammar document

Grammar must not describe omitted enum initializers as "automatic numeric continuation".
The canonical semantic rule is:

```text
first omitted initializer
    implicit initializer expression `iota`

later omitted initializer
    repeat the preceding initializer expression and evaluate it using the current `iota`
```

Example:

```sec
enum X int {
    A = iota,
    B,
    C,
    D = 10,
    E,
}
```

resolves to:

```text
A = 0
B = 1
C = 2
D = 10
E = 10
```

For continuation after a numeric jump:

```sec
enum X int {
    A = iota,
    B,
    C,
    D = iota + 7,
    E,
}
```

resolves to `0, 1, 2, 10, 11`.

Implementation-status text in `grammar.md` must be updated separately rather than changing
normative grammar to match the old parser behavior.
