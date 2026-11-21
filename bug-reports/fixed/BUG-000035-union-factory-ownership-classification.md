# BUG-000035 — Static factory mistaken for a union ownership constructor

## Status

Fixed 2026-11-21.

## Reproduction and evidence

`testdata/sema/union_ownership_forms_valid.sec` declares `Choice.Single(Handle)`
and an unrelated `Factory.Single(value: ref Handle) Choice`. Returning
`Factory.Single(value)` is an ordinary borrowed-argument call.

The pre-fix `TestUnionOwnershipOrdinaryFactoryIsOpaque` failed because
`unionConstructorPayload` classified that call as a constructor of `Handle`.
The helper checked the returned union, member spelling and whether the owner
named a type, without requiring the selected owner to be the union. This was
observed in Sema; native destruction behavior was not reproduced.

## Expected and actual behavior

Terminal ownership forwarding follows structural constructor edges. Ordinary
calls keep their parameter modes; a borrowed factory argument must not become
an owning union payload merely because the method has a variant-like name.
The old recognizer was used by terminal validation and move/resource traversal,
so its false classification could feed those paths an invalid ownership edge.

## Scope

- Phase: Sema ownership analysis.
- Governance: `frontend.union-constructor-value-semantics`.
- Rules: `rules/declarations/unions.md` §§4–5,7.8;
  `rules/memory/ownership.md` §17; `rules/memory/copy_move.md` §§9–10;
  `rules/corrections/applied/return-context-ownership-forwarding-correction-20260903.md` §1.

## Resolution

Named and unnamed payloads use the same typed initialization/ownership check.
Move/resource traversal and terminal-return validation consume saved constructor
operand edges, including before final transfer-action freezing. Ordinary calls
have no such constructor record and remain opaque to structural forwarding.
The remaining single-payload query also reads the saved record, not call spelling.
Paired valid/invalid tests cover copy, explicit/terminal move, fresh construction,
borrowed source, missing move, moved-source reuse and representability on both
scalar plans at every analysis depth. The factory regression checks the selected
call remains ordinary and its borrowed argument has no owning move plan.


## Verification

- The pre-fix recognizer regression failed with an ordinary factory classified
  as a `Handle` constructor. The fixed regression passes and finds no owning
  transfer plan on the factory's borrowed argument.
- Paired forms pass race-enabled Sema tests at Interactive, Standard and Deep
  depth on 32/64-bit plans. Invalid tests preserve the two valid constructors
  whose later moved-source reads fail, while rejected constructors publish none.
- A verified Semantic IR regression retains the ordinary caller's ownership:
  no owning move or cleanup cancellation is emitted for the borrowed factory
  call. Constructor traversal still uses saved operands after AST lists vanish.
- Complete Sema/IR/Sec MLIR emitter and traceability suites pass; existing
  source -> verified MLIR -> native union regression returns 42. Native
  destruction of the borrowed factory case itself is not claimed.
