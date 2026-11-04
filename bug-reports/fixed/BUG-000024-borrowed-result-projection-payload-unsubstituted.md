# BUG-000024 — `Result.OkRef`/`ErrRef` bindings get the unsubstituted payload type `T`

## Status

Fixed, 2026-11-04.

## Report

`result.OkRef` and `result.ErrRef` type-check as non-consuming projections
returning `Option[ref T]` / `Option[ref E]` (errorhandling.md § 6.2). A
`Some(v)` binding over that Option does not receive `ref int` for a
`Result[int, IndexError]`. It receives the bare generic parameter `T`.
Every use of the binding is therefore rejected, so the documented forms
`if result.ErrRef is Some(error)` and `Ok(value).OkRef -> Some(ref value)`
cannot be used.

`inferBorrowedResultProjection` (`internal/sema/analyzer_result_projection.go`)
builds `compilerKnownOption(compilerKnownSharedReference(payload))` from the
substituted type argument. The likely fault is that the Option payload pattern
reads the compiler-known `Option` variant payload without instantiating it
for this specialization. The consuming projections `.Ok()` and `.Err()` had
the same kind of fault, fixed by instantiating `member.Result`
(`analyzer_result_projection.go:117`). Triage confirmed this: see Resolution.

## Reproduction

```sec
module main

fn Get(value: int) Result[int, IndexError] {
    if value < 0 {
        return Err(IndexError.OutOfBounds)
    }
    return Ok(value)
}

fn Peek(value: int) int {
    let result := Get(value)
    let view := result.OkRef
    if view is Some(v) {
        return *v
    }
    return -2
}

fn Direct(value: int) int {
    let result := Get(value)
    if result.OkRef is Some(v) {
        let copy: ref int := v
        return *copy
    }
    return -2
}
```

```sh
bin/sec check okref.sec
```

## Expected and actual behavior

- **Expected:** `v` has type `ref int`, borrowing from `result`; both
  functions are accepted, and `*v` reads the Ok payload.
- **Actual (2026-11-04):**

  ```text
  sema error: operator * requires a safe reference operand, got T: it is not a safe ref T or ref mut T at okref.sec:14:16
  sema error: cannot initialize ref int with T at okref.sec:22:30
  ```

## Scope

- Area: Sema typing of compiler-known Result projections and Option payload patterns
- Rulebooks and corrections: `rules/errors/errorhandling.md` § 6.2
  ("Non-consuming borrowed projections"), § 29; `rules/memory/borrowing.md`
  (shared-borrow lifetime); `rules/compiler/compiler_known_members.md`
  (generic lookup and substitution)
- Governance integration: `frontend.copy-move` remaining item
  "result-borrowed-projections" in `governance/lifetime.yaml`;
  `frontend.errorhandling-v2` "result-projection-semantic-ir-lowering"
  (borrowed projection provenance) in `governance/errors.yaml`
- Affected phase: semantic analysis

## Resolution

The cause was confirmed. `compilerKnownOption` sets only `TypeArgs` on the
generic `Option` template, so its `Some` variant payload stayed `T`, and the
payload pattern read that variant. `inferBorrowedResultProjection` now
instantiates the Option (`a.instantiateGenericType(compilerKnownOption(borrowed))`),
as the consuming projections already did. A `Some(v)` binding over
`result.OkRef` is `ref int`, and one over `result.ErrRef` is `ref Failure`.
Receiver provenance is unchanged: returning the bound reference from a local
Result is still rejected (S1128), and a temporary receiver is still rejected
as not addressable.

Regression: `TestResultBorrowedProjectionPayloadBindingsAreSubstituted` in
`internal/sema/result_projection_test.go` covers if-bindings, a match over
`result.OkRef`, dereference, `ref` initialization and both escape rejections.
`go test ./internal/sema ./internal/ir/semantic ./internal/lowering/secmlir`
passes, and so does the LSP ownership diagnostic test that uses `OkRef`.

