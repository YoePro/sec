# Normative correction — Option versus Result API guidance

**Target:** `rules/errors/errorhandling.txt`  
**Status:** Applied
**Applied:** 2026-08-13
**Language version:** Sec 0.1  
**Created:** 2026-08-13  
**Last updated:** 2026-08-13

## Canonical API distinction

Use `Option[T]` when an operation is semantically valid but a normal input/domain
case may have no result and callers do not require a failure reason.

Use `Result[T, E]` when an operation can genuinely fail and the error reason is
part of correct handling or control flow.

Examples:

```sec
value.RemoveAt(index) -> Option[T]
map.Remove(key) -> Option[V]
vector.Normalize() -> Option[vector[...]]
```

For `Normalize`, a zero-magnitude vector yields `None`.

Examples requiring `Result` include storage/materialization operations whose
failure reason matters:

```sec
value.TransferTo(request) -> Result[T, StorageError]
view.Materialize(request) -> Result[T, StorageError]
```

This distinction is semantic, not based on whether an implementation happens to
have an internal failure branch.

APIs must not turn normal absence into `Error` merely because an error type could
be invented.

Documentation and LSP hover must show the exact return type and should state the
normal `None` condition or principal `Err` condition when that information is
needed to understand correct use.
