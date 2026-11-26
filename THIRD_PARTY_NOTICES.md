# Third-party notices — inventory pending

This is the entry point for Sec's third-party licensing information. It is not
yet a complete set of redistributable notices and must not be used as proof of
release compliance. The inventory is in
[third-party.yaml](governance/licensing/third-party.yaml).

| Component | Observed role | License information |
| --- | --- | --- |
| LLVM and MLIR | Native compiler backend / build dependency | Apache-2.0 WITH LLVM-exception is the principal upstream license; exact build version, legacy and bundled notices must be checked. |
| Go | Bootstrap toolchain; runtime and standard library may enter distributed binaries | BSD-3-Clause, separate PATENTS grant and component-specific notices; confirm exact toolchain. |
| Go modules | Dependencies listed in go.mod / go.sum and the optional cmd/sec-mcp module | Version-specific licenses and notices pending. |
| Unicode | Data under third_party/unicode and potentially derived tables | Verify original release notices and preserve source attribution. |
| VS Code extension | Dependencies in vscode/package-lock.json | Version-specific licenses and actual VSIX contents pending. |
| Native libraries, assets and generated artifacts | Target- and distribution-specific | Inventory and provenance review pending. |

Upstream references:
- https://llvm.org/docs/DeveloperPolicy.html#copyright-license-and-patents
- https://github.com/llvm/llvm-project/blob/main/LICENSE.TXT
- https://go.dev/LICENSE
- https://go.dev/PATENTS
- https://www.unicode.org/license.txt

For each release, collect actual version-specific license, copyright, patent
and required NOTICE files in a distribution notice directory, together with an
artifact-specific inventory. Include notices with binary archives and extension
packages when required. The license templates in LICENSES/ are not substitutes
for third-party notices. Do not remove or relicense existing external files.
