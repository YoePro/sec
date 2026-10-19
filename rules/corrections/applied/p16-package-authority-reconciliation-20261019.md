# Correction: P16 package authority reconciliation

- **Status:** Applied 2026-10-19
- **Created:** 2026-10-19
- **Document revision:** 1
- **Work item:** P16-03
- **Authority:** current canonical source and representation rules, integrated
  by P16-04 and P16-05; MD-074/MD-088 unsigned-length correction
  (2026-10-17); P15 § 28 address-stability revision (2026-10-03)
- **Targets:** `rules/mlir/packages/sec-mlir-dialect_package16.md`,
  `rules/mlir/packages/sec-mlir-dialect_package16.yaml`

## Audit and reconciliation

The package's new current-authority table records the review of P16 §§ 1–2,
all four supplied inputs and the owning current source, memory, error, core,
Semantic IR and MLIR contracts. The original August repository/version fields
remain historical provenance, not a claim about current main or a clean
execution snapshot.

The package prose and machine-readable contract are synchronized:

- source spelling is `.Len`, global `len(slice)` returns `uint`, both use
  `SliceLengthOp` / `sec.slice.len`, and the old signed operation is removed
  from operation inventories;
- published sections 48, 49 and 101 are replaced in place with the accepted
  unsigned-length contract, preserving all 146 numbered package sections;
- target-resolution descriptions, required tests, acceptance/report wording
  and unsupported inventories no longer require a signed-length proof or
  describe the already resolved representability decision as open;
- `RelocationClass` is replaced by canonical address stability plus separate
  pin dependency, including the illustrative facts and P15 reuse inventory;
- executable Semantic IR version 1 is distinguished from rulebook document
  revision 2.0 and the schema/lowering-12 specification; representation
  installation does not promote default schema-11 emission;
- existing source-rule decisions for protected try subtrees (MD-028), explicit
  reborrow (MD-061), Place display (MD-089) and constant-range diagnostics
  (MD-094) remain owned by their canonical books and applied corrections.

No unsafe operation, raw conversion, physical descriptor, owning-array
representation or alternative compilation route is introduced. The history
snapshots retain their original text with the P16-05 integration notices.

## Remaining decisions and acceptance

Semantic model drift is resolved; concrete metadata serialization (MD-084)
and returned-origin verification obligations (MD-085) remain explicitly open.
Their existing gates cannot be closed by picking spellings in a package
checklist. P16-01 snapshot capture, P16-02 execution gates, P16-07 predecessor
acceptance and outstanding source/IR/MLIR implementation remain independent.

The package YAML is parsed and checked against the corrected length and
operation inventories, all 146 numbered Markdown sections are preserved, and
`language-rulebook-status.md` and owning P16 governance are synchronized.
