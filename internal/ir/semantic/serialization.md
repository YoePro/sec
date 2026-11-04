# Semantic IR JSON transport

`sec emit-ir input.sec --json [-o module.sir.json] [--target os-arch]`
publishes the verified Semantic IR of the ordinary compilation track. Without
`--json`, `emit-ir` retains its human-readable text output. Diagnostic JSON
continues to be a separate stderr document selected by `--diagnostic-format=json`.

The transport is an internal inspection/interchange format governed by
`rules/compiler/semantic_ir.md` §§97,106–107. There is no long-term external
compatibility guarantee. Its header is:

```json
{
  "format": "sec.semantic-ir",
  "version": 1,
  "module": {
    "Version": 1,
    "Identity": "example",
    "Types": []
  }
}
```

The example is a minimal empty module. Compiler output includes all exported
`semantic.Module` facts with their Go field names. `Types` is an ordered array:
the first element has TypeID 1, the second TypeID 2, and so on. Operations,
definitions, ownership/cleanup plans, source positions, exact integer/decimal
constants, reference metadata and selected pointer width retain their existing
Semantic IR representations. No AST or Sema object is serialized. Ordered slices
retain their order; repeated serialization of the same module produces the same
bytes and a final newline.

Use `semantic.Serialize(module)` to verify and encode a module, and
`semantic.Deserialize(data)` to decode it. Direct `json.Unmarshal` into Module
cannot reconstruct the private type-interning index and is not a supported
loading path. Deserialization rebuilds that index without changing any TypeID,
rejects duplicates, then runs the ordinary complete Semantic IR verifier. A
failure returns no module. Unknown fields, duplicate object keys (including
case-folded aliases), extra JSON values, invalid UTF-8 and incompatible versions
are rejected.

There are three independent version identities:

- Envelope `version`: `semantic.SerializationVersion`, currently 1.
- Module `Version`: the operation/type/invariant schema, `semantic.Version`, 1.
- Sec MLIR dialect schema: controlled by the existing lowering pipeline.

An incompatible transport change requires an envelope version change. An
incompatible Semantic IR operation/type/invariant change requires a module
schema change. Neither version is inferred from a rulebook document revision.
Both executable versions must match when loading; no implicit migration occurs.

This transport does not introduce a compilation cache. Source snapshot and
CompilationPlan freshness, cache identity and reuse admission remain the
responsibility of the owning compiler workspace/cache. A successfully decoded
module retains the plan facts it was built with; decoding does not retarget it.
The CLI builds and serializes fully before publishing its output, so a frontend,
IR construction or serialization failure does not publish partial JSON.
