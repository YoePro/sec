# Contributing to Sec

Sec welcomes bug reports, design proposals, tests, documentation and code.
Discuss material language or architecture changes before implementation;
maintainer approval is required for changes to normative language decisions.

## Workflow

1. Open or identify the relevant issue, rulebook or approved decision.
2. Work on a separate branch. Keep changes focused and avoid unrelated edits.
3. Cite the governing rulebook or decision in semantic implementation changes.
   Update affected tests, documentation and canonical governance entries together.
4. Run applicable tests (`go test ./...` for the Go bootstrap compiler, plus
   affected MLIR, Sec and editor tests as appropriate). Report commands, results
   and any tests not run. Include a minimal reproducer for compiler bugs.
5. Open a pull request with rationale, references, test results and licensing
   or dependency changes. Address review comments before merge.

## License of contributions

By submitting a contribution for inclusion, you offer the rights you control
under the applicable license described in [LICENSE.md](LICENSE.md): MIT for
software, CC BY 4.0 for specification and documentation, and both alternatives
for original embedded code examples. You retain your copyright. No separate
copyright assignment or contributor license agreement is required under this
policy. Do not submit material you lack permission to contribute. Obtain any
necessary employer or co-author permission first. Imported material must retain
its original terms and requires explicit maintainer review.

## Developer Certificate of Origin (DCO)

Every new contributor commit must carry the contributor's own sign-off under
[DCO 1.1](DCO), including documentation contributions. For documentation, the
applicable license is CC BY 4.0 as described above. A sign-off certifies origin
and authority, not a transfer of ownership or a separate patent license.

```sh
git commit -s -m "Describe the change"
```

A forgotten sign-off on your own latest commit can be added before submission:

```sh
git commit --amend -s --no-edit
```

Use a name and email that identify you as the contributor; a GitHub noreply
address may be used. Sign-off is not cryptographic signing. Never sign on
another person's behalf. Preserve applicable sign-offs and authorship when
forwarding or combining contributions. For older commits predating adoption,
do not rewrite history or fabricate certifications. Review provenance instead.

## Third-party material and AI assistance

Disclose external source, exact version, license, copyright/notice files,
modifications and intended use. Obtain approval before adding a dependency,
copying snippets, or importing datasets, assets or generated tables.

AI assistance is allowed only when the human submitter can responsibly review,
test and establish the right to submit the contribution. Disclose material
AI assistance in the PR and describe validation performed. Do not invent AI
sign-offs, authorship or rights. Do not paste confidential material or code
with unknown licensing. Substantial similarity to external code requires
provenance review; uncertainty blocks acceptance until resolved.

Disclose known patent claims or restrictions relevant to a contribution to the
maintainers without publishing confidential legal advice. No one is asked to
certify that no third-party patent exists.

See [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) and the
[licensing policy](governance/licensing/policy.md).
