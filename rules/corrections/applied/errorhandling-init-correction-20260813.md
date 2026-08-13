# Correction: error handling for fallible lifecycle construction

**Target:** `rules/errors/errorhandling.txt`  
**Source rule:** `rules/declarations/impl.md` revision 2.0  
**Date:** 2026-08-13

## Fallible `init`

A lifecycle initializer may declare an exact construction error type:

```sec
impl Connection {
    init(address: Address) ConnectError {
        // ...
    }
}
```

`ConnectError` is not a normal function return type.

It is the error channel for construction.

Successful construction produces the impl target instance, not an `Ok` payload
and not the error type.

For control-flow/error analysis, a fallible `init` behaves like an operation
with:

```text
success payload: none (the completed instance is implicit)
error type: E
```

The body may use ordinary Sec `try` propagation only when the propagated error
type matches the initializer's declared error type exactly, unless the error is
handled/mapped locally according to ordinary error rules.

An initializer must not return an arbitrary success value.

## `new` fallibility

For:

```sec
let connection := try new Connection(address)
```

on success:

```text
connection: Connection
```

On failure, the selected initializer's declared error type is propagated or
handled according to ordinary `try` semantics.

A fallible `new` without `try`/handling is invalid in the same way other explicit
fallible operations must be handled.

## Local handlers

Where the grammar permits ordinary `try` handlers, a fallible construction may
be handled locally:

```sec
fn Open(address: Address) Result[Connection, ConnectError] {
    let connection := try new Connection(address) {
        Err(error) => return Err(error)
    }
    return Ok(connection)
}
```

The success path remains the constructed `Connection` instance. In value
context, local handlers follow the ordinary rule that an error arm must return,
terminate, propagate, or produce a fallback `Connection`.

## Construction failure edge

Sema, ownership, definite-initialization, destruction, escape, effect, Semantic
IR, and lowering must model initializer failure as an early construction-failure
edge:

- no completed instance is produced;
- partial-construction cleanup executes;
- completed-value `free` does not execute;
- exact error typing is preserved.

## No `Option[E]` error channel

Do not model fallible `init` as `Option[E]`.

`Option[T]` remains the normal absence/domain model; construction failure with a
meaningful typed reason uses the initializer's explicit error channel and normal
Sec error semantics.
