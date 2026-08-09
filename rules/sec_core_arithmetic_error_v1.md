# Package 9 Normative Core Error Addition

## Target rulebook

Apply this normative addition to:

```text
rules/core-library.md
```

The purpose is to close the current mismatch where `runtime_checks.md` defines
fallible checked arithmetic in terms of `ArithmeticError`, while the minimum
core error list does not yet define that error's exact value set.

---

# Canonical core type

Add to the required standard core errors:

```sec
enum ArithmeticError {
    Overflow
    DivisionByZero
    InvalidShift
}
```

`ArithmeticError` is always available.

It is a normal exact named core error type.

It participates in:

```text
Result
try
match
explicit error mapping
```

It does not implicitly convert to:

```text
OverflowError
DivisionByZeroError
another user error
a union containing ArithmeticError
```

and those errors do not implicitly convert to it.

---

# Canonical builtin arithmetic mapping

```text
integer negation overflow
integer addition overflow
integer subtraction overflow/underflow
integer multiplication overflow
signed minimum / -1
signed minimum % -1
signed left-shift representability failure
    -> ArithmeticError.Overflow

division by zero
remainder by zero
    -> ArithmeticError.DivisionByZero

negative shift count
shift count >= value bit width
    -> ArithmeticError.InvalidShift
```

If signed left shift has an invalid count, `InvalidShift` takes precedence over
the representability check because the mathematical shift is not valid.

If signed division/remainder has zero divisor, `DivisionByZero` takes precedence
over the min/-1 check.

---

# Existing narrower core errors

Existing:

```text
OverflowError
DivisionByZeroError
```

remain valid core types for APIs whose contract specifically returns those
narrow errors.

They are not removed.

Builtin fallible operator arithmetic uses the unified:

```text
ArithmeticError
```

required by `runtime_checks.md`.

---

# Implementation status update

When the core source is updated, add `ArithmeticError` to the list of core errors
registered before user semantic analysis.

Do not describe it as a standard-library error.

It is a core language-operation error.

---

# No runtime representation requirement

The source enum does not imply:

```text
heap allocation
exception object
reflection
runtime registry
unwinding
```

It is an ordinary statically known core error value.

The exact low-level enum width is a later representation decision.
