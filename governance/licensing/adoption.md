# Adoption checklist

Revision: 1 | Date: 2026-11-25 | Status: pending

Before publishing the license rollout:

- [ ] Confirm the legal copyright holder(s), attribution name and years. The
  initial LICENSE uses `2026 Jonas`, matching vscode/LICENSE, not a verified
  repository-wide ownership statement. Confirm rights to prior contributions,
  including any third-party or AI-assisted material, before applying licenses.
- [ ] Confirm license scope in LICENSE.md, including the proposed code-example
  dual option, and DCO/contribution and conduct policies. Review mixed files,
  assets and any historical documents individually.
- [ ] Add SPDX attribution to verified files, expanding REUSE.toml carefully.
  Keep copyright statements for existing upstream material. Establish an
  appropriate metadata treatment for the verbatim DCO and root license files.
- [ ] Add a licensing section to the existing root README linking LICENSE.md,
  CONTRIBUTING.md and THIRD_PARTY_NOTICES.md. Add a pointer from governance/README.md
  if desired, without adding a fragment to the implementation index.
- [ ] Configure a DCO check for new PR commits and require it in branch rules.
  A GitHub App or reviewed CI integration can do this; installing it and enabling
  protection require maintainer action. Do not rewrite old commits.
- [ ] Resolve the dependency inventory on the actual build host. Collect exact
  upstream texts and notices, including separate patent grants and exception
  texts where relevant. Keep version-specific notices with each distribution.
- [ ] Run `reuse lint`, remediate the existing tree and enable CI validation.
  The initial configuration is intentionally incomplete and not a passing baseline.
- [ ] Define release packaging and verify each artifact's notices and SBOM.
- [ ] Schedule patent review separately; do not imply clearance.

## Inventory commands (run from repository root)

```sh
go version
go list -m -json all
go list -deps -test -json ./...
llvm-config --version
npm --prefix vscode ls --all --json
```

Run Go and native dependency checks for the actual build targets. Install the
locked npm graph in a reviewed environment before querying it if needed.
Inspect released binaries with `go version -m <binary>` where supported and
native linkage tools appropriate to the target. Inspect final VSIX contents.
These commands provide inventory inputs, not license determinations.

## License text sources

- MIT: https://spdx.org/licenses/MIT.html
- CC BY 4.0: https://creativecommons.org/licenses/by/4.0/legalcode.en
- DCO 1.1: https://developercertificate.org/
- REUSE specification: https://reuse.software/spec-3.3/

MIT and CC BY texts in this package were copied from locally available standard
license text files; direct download was unavailable during preparation. The
root MIT copy fills the standard copyright field. DCO is the standard text.
Compare with the referenced originals during final adoption if desired.

Do not treat this scaffold as a completed license audit or patent analysis.
