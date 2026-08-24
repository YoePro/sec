# If correction — Option presence binding with `is Some(binding)`

- **Status:** Applied normative correction
- **Applied:** 2026-08-24
- **Created:** 2026-08-24
- **Last updated:** 2026-08-24
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `b3315f6` (semantic parent `45e5cd4`)
- **Target rulebook:** `rules/control-flow/flowcontrol_if.md`

---

## Correction

Retain the general rule that `if` is not a pattern-matching construct, but add
one narrow Sec 0.1 Option convenience exception.

### Non-binding tests

These remain valid non-binding state tests:

```sec
if option is None {
    ...
}
```

```sec
if option is not None {
    ...
}
```

### Positive `Some` binding

Permit:

```sec
if option is Some(value) {
    Use(value)
}
```

The binding:

- is introduced only on the true branch;
- has the Option payload type;
- follows ordinary copy/move ownership rules for the subject form;
- participates in path-sensitive ownership merge;
- is not visible in `else` or after the `if` unless ordinary outer state permits it.

This is a language-defined Option exception, not general union destructuring in
`if`.

### Negative binding is invalid

Do not permit:

```sec
if option is not Some(value) {
    Use(value)
}
```

because the true path does not contain a `Some` payload to bind.

### Other payload patterns

General payload/structural destructuring remains owned by `match`.
This correction does not make `if result is Ok(value)` generally valid merely
because `Some(value)` is valid.

## Cross-reference

```text
rules/errors/errorhandling.md
rules/control-flow/flowcontrol_match.md
```
