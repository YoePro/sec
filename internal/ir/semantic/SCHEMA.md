# Semantic IR schema ownership

The `semantic` Go package owns the executable Semantic IR schema. Its single
version authority is `semantic.Version`, declared in [model.go](model.go).
`Module.Version` identifies the schema of a particular module. The canonical
requirements are [Semantic IR §§ 103–107](../../../rules/compiler/semantic_ir.md).
This document describes implementation maintenance, not additional language
semantics.

## Independent version domains

| Domain | Authority | Meaning |
| --- | --- | --- |
| Semantic IR executable schema | `semantic.Version` in `model.go` | Compatibility of typed modules and their invariants |
| Semantic IR rulebook revision | Header of `rules/compiler/semantic_ir.md` | Revision of the normative document |
| Sec MLIR output schema | `emissionSchema` and `dialectSchemaVersion` in `internal/lowering/secmlir/` | Dialect operations admitted at the emission boundary |
| MLIR work package | `rules/mlir/packages/` and the owning governance integration | Implementation scope and predecessor gates |

Neither a document revision nor a completed package automatically changes an
executable schema or selects a newer emission schema. The Semantic IR version
is not the Sec MLIR dialect version. Avoid copying a numeric Semantic IR version
into a new producer or consumer: use `semantic.Version`.

## Acceptance boundaries

The builder stamps `Module.Version` from `Version`. `semantic.Verify` rejects
every other version before inspecting module contents. `secmlir.Emit` and
`EmitWithOptions` verify the module before emission and write
`sec.semantic_ir_version` from the same authority. Selecting a newer dialect
schema cannot authorize an incompatible Semantic IR module.

`semantic.Format` is diagnostic output, not a persisted format or a validating
reader. There is currently no serialized Semantic IR loader. Do not treat the
printer as a compatibility protocol or claim persisted-module support from the
in-memory verifier. Any future loader must validate its schema and the complete
compiler/rulebook/CompilationPlan dependency identity before publishing facts;
matching `Version` alone is insufficient for cache reuse (§ 105).

## Schema-change procedure

For an incompatible change to operation meaning, types, invariants or
serialization semantics (§ 106(2)):

1. Establish the canonical rulebook requirement or correction first. A work
   package report is implementation history, not a separate semantic authority.
2. Increment `Version` in `model.go` in the same change as the incompatible
   producer, verifier and consumer changes. Do not reinterpret modules carrying
   the old number. There is no implicit upgrade or downgrade path.
3. Update the builder, model, verifier, printer and affected lowering consumers
   together. Change a dialect schema only if its own compatibility contract also
   changes; do not couple the two version numbers.
4. Test a newly built module, incompatible/unspecified versions, and the affected
   operations and lowering boundaries. Retain rejection tests for old inputs;
   do not merely regenerate expected output to conceal incompatibility.
5. Synchronize executable-version mentions in the canonical rulebook (including
   § 106(5)), the rulebook status index when its document changes, and the owning
   governance integration. Record focused verification and remaining boundaries.

A compatible implementation extension does not require a bump solely because
another package is completed. Its review must still check existing invariant
and consumer compatibility. This is not permission to retain a number after an
incompatible semantic change.

The governance owner of this policy is `governance.semantic-ir-v2` in
`governance/governance.yaml`. Implementation completion belongs to the existing
semantic or lowering owner; do not duplicate its checklist here.

The complete [package amendment crosswalk and maintenance workflow](../../../governance/semantic-ir-amendments.md)
maps retained package sections to canonical semantic responsibilities and the
live implementation owners. Use it when a package changes this schema; package
implementation inventories and acceptance reports stay outside the canonical
semantic rulebook.
