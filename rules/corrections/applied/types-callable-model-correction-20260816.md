# Correction: callable types after functions/lambda v2

- **Status:** Applied 2026-08-16
- **Created:** 2026-08-14
- **Last updated:** 2026-08-14
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Target:** `rules/types/types.md`

## Required correction

The function-type overview must preserve the complete callable contract rather than only ordinary parameter and return types.

Canonical callable capabilities are:

```text
fn       shared/reusable callable
mut fn   mutable/exclusive callable
-> fn    consuming/one-shot callable
```

Examples:

```sec
fn(int) bool
mut fn() int
-> fn() Resource
```

Callable parameter modes are also part of the callable type.

Examples:

```sec
fn(ref Buffer) void
fn(ref mut Buffer) void
fn(-> Buffer) void
fn(...int) int
```

Callable capability and parameter consumption are distinct.

For example:

```sec
-> fn(-> Resource) Handle
```

means:

- the callable value is consumed when invoked;
- the `Resource` argument is also consumed.

Parameter names are not part of callable type identity.

The complete callable type identity/compatibility model includes at least:

- callable capability;
- parameter count;
- ordered parameter types;
- parameter borrow/ownership modes;
- variadic shape and element type;
- return type.

Capture contents are not individually written into source-level callable type syntax, but their semantics may determine callable capability, copy/move behavior, lifetime, and whether the callable may cross a particular ABI boundary.

The detailed callable semantics remain owned by `functions.md` and `lambda-functions.md`.
