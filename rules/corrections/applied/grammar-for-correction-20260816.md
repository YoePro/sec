# Grammar correction: `for` reference bindings

- **Status:** Applied normative correction
- **Applied:** 2026-08-16
- **Created:** 2026-08-16
- **Last updated:** 2026-08-16
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical correction path:** `rules/corrections/grammar-for-correction.md`
- **Source:** `rules/control-flow/flowcontrol_for.md` revision 2.0
- **Target:** `rules/foundations/grammar.md`
- **Repository baseline reviewed:** `56be75d`

---

## Required correction

The current canonical grammar limits `ForBinding` to an identifier or `_`.

Update the iterable-loop grammar so it can represent the explicit ownership modes defined by `flowcontrol_for.md` revision 2.0.

Replace:

```text
ForBinding
    ::= Identifier
      | "_"
```

with:

```text
ForBinding
    ::= Identifier
      | "_"
      | "ref" Identifier
      | "ref" "mut" Identifier
```

The surrounding form remains:

```text
IterableForStatement
    ::= "for"
        ForBinding { "," ForBinding }
        "in"
        IterableExpression
        [ Contextual("step") Expression ]
        Block
```

## Semantic restriction remains outside grammar

The grammar accepts binding forms syntactically.

Semantic analysis determines whether a binding mode is legal for its iterable and binding position.

Examples include:

```text
sequential index binding
    Identifier or _ only

sequential element binding
    value / ref / ref mut / _ according to source authority

map key
    value / ref / _
    ref mut invalid

map value
    value / ref / ref mut / _ according to source authority

set value
    value / ref / _
    ref mut invalid

range value
    value / _ only

string rune
    value / _ only
```

## No consuming grammar

Do not add:

```text
"->" ForBinding
"move" IterableExpression
```

or another consuming-loop grammar form for Sec 0.1.

Consuming `for` iteration is not part of Sec 0.1.
