# Default-values correction — enums

**Status:** Applied
**Applied:** 2026-08-13
**Language version:** Sec 0.1
**Created:** 2026-08-13
**Last updated:** 2026-08-13
**Target:** `rules/types/default_values.md`
**Source of truth:** `rules/declarations/enums.md`

## Replace the enum placeholder with the following rule

Every valid enum is non-empty and therefore defaultable.

The enum default is selected by declared member identity, not by underlying numeric zero.

Resolution order:

```text
1. the single member marked `default`, if present;
2. otherwise the first declared member.
```

Examples:

```sec
enum Status int {
    UNKNOWN = 10,
    ACTIVE = 20,
}
```

Default:

```sec
Status.UNKNOWN
```

Explicit override:

```sec
enum ConnectionState {
    CONNECTING,
    CONNECTED,
    DISCONNECTED default,
}
```

Default:

```sec
ConnectionState.DISCONNECTED
```

The `default` marker does not modify the member initializer, `iota`, aliases, or initializer
repetition.

For a bit-backed open enum, the default must still be a declared member. An undeclared raw
bit pattern is never selected merely because its numeric representation is zero.

A mutable enum declaration without an initializer is semantically default-initialized:

```sec
let mut state: ConnectionState
```

is equivalent to initialization with the resolved enum default.

This supersedes any older rule describing such an enum variable as uninitialized storage
requiring definite assignment before first read.
