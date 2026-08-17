# Grammar correction — `match` patterns and contextual arm values

- **Status:** Applied normative correction
- **Applied:** 2026-08-17
- **Created:** 2026-08-17
- **Last updated:** 2026-08-17
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `56be75d`
- **Target rulebook:** `rules/foundations/grammar.md`

---

## Correction

The canonical Sec 0.1 grammar for `match` must be synchronized with `rules/control-flow/flowcontrol_match.md` revision 2.0.

### Match must contain at least one arm

Replace the zero-or-more arm form with a grammar that requires one or more arms.

Conceptually:

```text
MatchExpression
    ::= "match" Expression
        "{"
        MatchArm { MatchArm }
        "}"
```

`match value {}` is invalid in Sec 0.1.

### Canonical pattern categories

The grammar must not document general literal matching as part of the canonical Sec 0.1 match-pattern language.

Remove `LiteralPattern` from the canonical match grammar.

Direct `true` and `false` match patterns are not Sec 0.1 match syntax.

Ordinary literal and range selection belongs to `switch`.

Enum member patterns remain valid because they are resolved enum-member patterns, not general literal patterns.

### `empty`

Add the compiler-known union initialization-state pattern:

```text
EmptyPattern
    ::= "empty"
```

`empty` is valid only where Sema proves that the matched union binding may expose the compiler-known empty initialization state.

### Whole-payload binding modes

Pattern payload bindings must preserve these canonical forms:

```text
PayloadBinding
    ::= Identifier
      | "_"
      | "ref" Identifier
      | "ref" "mut" Identifier
```

The parser preserves spelling. Sema decides whether the binding copies, moves, borrows, mutably borrows, discards, or is invalid.

### Shallow struct-like union-field destructuring

Field-level payload destructuring is part of Sec 0.1 and must no longer be documented as postponed.

Conceptual grammar:

```text
StructLikeUnionPattern
    ::= QualifiedIdentifier
        "{"
        [ FieldPattern { "," FieldPattern } [ "," ] ]
        "}"

FieldPattern
    ::= Identifier
      | Identifier ":" Identifier
      | Identifier ":" "ref" Identifier
      | Identifier ":" "ref" "mut" Identifier
```

The semantic model is shallow.

The grammar must not imply recursive nested pattern support.

Partial field binding requires no `..` marker.

### Match arm body

Keep:

```text
MatchArmBody
    ::= Expression
      | ReturnStatement
      | Block
```

but document that a `Block` in expression-match result position has contextual match-arm value semantics:

- the final expression on every continuing path is the arm value;
- terminating paths need no arm value;
- this does not create general block-expression syntax elsewhere in Sec.

### Remove bool-match example

Remove grammar examples equivalent to:

```sec
let number := match condition {
    _ where condition => 1
    _ => 0
}
```

when the subject is merely a `bool` condition.

Use `if` / `else` for direct boolean branching.

## Cross-reference

Canonical semantics are defined by:

```text
rules/control-flow/flowcontrol_match.md
```
