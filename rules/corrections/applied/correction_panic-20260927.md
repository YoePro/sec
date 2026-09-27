# Correction — Explicit panic syntax

* **Status:** Applied normative correction
* **Created:** 2026-09-27
* **Last updated:** 2026-09-27
* **Applies to:** `rules/errors/panic.md`, revision 2.0
* **Scope:** § 17(1)–§ 17(3)

This correction replaces § 17(1)–§ 17(3) only. All other panic semantics remain unchanged.

## Corrected § 17 Explicit panic

### § 17(1) Canonical syntax

Sec 0.1 defines the following explicit-panic forms:

```sec
panic
panic "message"
```

Equivalently, the syntax may be written:

```text
panic ["message"]
```

where `[...]` denotes an optional grammar element and is not part of Sec source syntax.

### § 17(2) Grammar

```text
panic_statement :=
    "panic" [ string_literal ]
```

`panic` is a statement keyword, not an ordinary callable. Function-like forms such as:

```sec
panic("message")
```

are invalid.

### § 17(3) Optional message

The panic message is optional.

When present, it must be an ordinary string literal in Sec 0.1. Arbitrary runtime expressions, dynamically computed strings, and interpolated messages are not permitted.

Omitting the message does not change the semantics of explicit panic; it only omits the optional diagnostic message metadata.
