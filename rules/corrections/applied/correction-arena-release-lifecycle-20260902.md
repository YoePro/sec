# Correction: compiler-known terminal lifecycle operations

- Status: Applied
- Created: 2026-09-02
- Applied: 2026-09-02
- Sec language version: 0.1
- Applies to:
  - `rules/memory/ownership.md`
  - `rules/memory/destruction.md`
- Triggering rulebook:
  - `rules/memory/arena.md`

---

## Problem

Current ownership/destruction rules correctly forbid ordinary user-defined methods from
consuming whole `self`.

They also establish `free` as the ordinary user-defined whole-value lifecycle cleanup
mechanism.

Arena has an existing canonical source operation:

```sec
arena.Release()
```

whose semantics terminate the ArenaDomain and consume the Arena owner.

Treating `Release()` as an ordinary consuming user-defined method would conflict with the
ownership/destruction rules.

Removing or renaming the established Arena operation is not required.

---

## Correction

The prohibition applies to ordinary user-defined methods.

A semantic builtin type may define a compiler-known terminal lifecycle operation when all
of the following hold:

1. the operation is explicitly defined by a canonical rulebook;
2. the compiler owns its ownership transition;
3. the operation is terminal for the source owner;
4. no continuing usable whole-`self` value is produced;
5. automatic destruction can recognize that the value has already been consumed;
6. the operation does not create a general source-language facility for user-defined
   consuming methods.

For Sec 0.1, `Arena.Release()` is such an operation.

```sec
arena.Release()
```

After successful `Release()`:

```text
the Arena owner is consumed;
the ArenaDomain is terminated;
the source Arena Place is unavailable;
no later operation may use that Arena value;
automatic destruction must not perform a second Release.
```

`Arena.Release()` is therefore not precedent for declarations such as an ordinary
user-defined:

```sec
fn Close(<-self) void
```

or any equivalent whole-self-consuming method syntax.

User-defined whole-value cleanup remains governed by `free` and the ordinary destruction
rules.

---

## Required synchronization

When this correction is applied:

- `ownership.md` should state that its whole-self method prohibition applies to ordinary
  user-defined methods and does not prohibit canonically defined compiler-known terminal
  lifecycle operations.
- `destruction.md` should state that a compiler-known terminal lifecycle operation may
  consume a builtin owner and suppress later automatic destruction of that consumed value.
- `arena.md` remains authoritative for the exact `Arena.Release()` lifecycle semantics.

This correction introduces no general consuming-method feature.
