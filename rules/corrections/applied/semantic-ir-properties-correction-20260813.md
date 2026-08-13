# Correction: explicit property operations in Semantic IR

- **Status:** Applied 2026-08-13
- **Created:** 2026-08-13
- **Last updated:** 2026-08-13
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Target:** Semantic IR rulebook

Property access must remain semantically explicit through Semantic IR.

The IR must distinguish at least:

- property read,
- infallible property write,
- fallible property write,
- compound property update or an equivalent expanded sequence with guaranteed single evaluation.

The IR must preserve:

- receiver evaluation order,
- getter/setter side effects,
- ownership and borrow effects,
- fallible setter error edges,
- cleanup and defer behavior.

Backends must not infer property behavior from ordinary field access.
