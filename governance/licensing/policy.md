# Sec licensing policy

Revision: 1 | Date: 2026-11-25 | Status: proposed for adoption

## 1. Scope and classification

Apply the first-party policy in ../../LICENSE.md to original work for which the
relevant rights holders have authorized licensing. Classify files by content,
not merely by extension or directory. Source code, compiler fixtures, machine
readable grammar, build scripts, and runtime are MIT; original specification
and explanatory prose are CC BY 4.0. Original embedded code examples have an
additional MIT option. Machine-readable normative documents require review of
whether they are specification or implementation artifacts. Mixed works require
explicit identification of the boundaries. Do not infer ownership from a commit
or relicense imported or generated material automatically.

## 2. Metadata

Use SPDX license identifiers and accurate copyright attribution per REUSE 3.3.
For files that support comments, use the appropriate SPDX headers; for other
files use REUSE.toml annotations or sidecars. Prefer narrow paths; retain
upstream headers. Record separate licensing for embedded material when necessary.
Check generator inputs and outputs independently. Do not add placeholder
copyright identities to published works. Keep canonical texts in LICENSES/.

## 3. Third-party intake

Record upstream source, exact version/commit and hashes, component use, file
scope, dependency relationship, license expression, original notices, patent
terms where relevant, modifications, distribution targets and reviewer decision.
Review direct, transitive, build-only, test, runtime, vendored, generated and
asset dependencies. A package-manager license field is a hint, not evidence.

MIT, BSD-2-Clause, BSD-3-Clause, ISC, Apache-2.0, Apache-2.0 WITH LLVM-exception,
Zlib and Unicode licenses are candidates for routine review, not automatically
approved. Other licenses, dual-license choices, copyleft, noncommercial,
source-available or unknown terms require explicit approval and compatibility
analysis for the intended use. No dependency with unresolved obligations is
approved for distribution. Preserve all required notices and satisfy applicable
source, modification, patent and redistribution conditions.

## 4. Release controls

For every source, binary, installer, VSIX or target runtime distribution, resolve
actual included components, including statically linked code, generated tables,
LLVM distributions and native/system runtimes as applicable. Collect original
notices, license texts and any applicable source offers or other materials.
Generate a versioned SPDX or CycloneDX SBOM for the artifact and retain build
provenance; a SBOM is not a replacement for notices. Verify package contents.
CI should validate new licensing metadata and dependency changes; enable full
REUSE and release gates only after baseline remediation, without claiming the
current tree passes. Hold releases with unresolved licensing blockers.

## 5. Contribution and patents

Use the DCO workflow described in CONTRIBUTING.md after maintainer adoption.
Review existing contributions before first-party license rollout. Preserve
copyright holders' rights. Patent review is a separate process; a license scan
never establishes freedom to operate. See patent-policy.md.
