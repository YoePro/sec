# BUG-000022 — Semantic IR null blocks panic before structural verification

Fixed, 2026-11-04. Owner: `tooling.semantic-ir-v2` in `governance/tooling.yaml`.

## Reproduction and cause

The new serialized-IR corruption matrix replaces a function's first block with
JSON `null`. `Deserialize` invokes the ordinary `semantic.Verify`, which calls
`verifyStaticPlaceIdentities` before the per-function structural verifier. Its
block traversal dereferenced the null block and panicked instead of returning
an error. `verifyFunction` already had a nil-block rejection, but ran too late.
The same ordering affected null blocks inside nested defer regions.

Reproduced during implementation with:

```sh
go test ./internal/ir/semantic -run TestSemanticIRSerializationRejectsInvalid -count=1
```

The stack identified `static_place_identities.go`, the block operation traversal.
Rules: `rules/compiler/semantic_ir.md` §§96–97 require verification and rejection
of invalid IR at the generation, serialization and deserialization boundaries.

## Fix and regression evidence

The shared static Place identity traversal checks for null blocks before reading
operations, including recursive region traversal. It returns a verifier error;
no second deserialization-specific verifier or recovery from panic is used.
`Deserialize` publishes no module and `Serialize` publishes no data on failure.

`TestSemanticIRSerializationRejectsInvalid/null_block` covers malformed loaded
IR. `TestSemanticIRSerializationRejectsNullRegionBlock` covers direct Verify and
Serialize of a malformed nested cleanup region. Full Semantic IR tests and the
focused serialization/CLI race tests verify the fix. No source-language,
ownership, destruction, defer ordering or lowering semantics changed.
