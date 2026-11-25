# Reviewable C-binding edits

The existing mutation commands also support a read-only plan:

```sh
sec platform c-bindings environment set linux-amd64-glibc \
    --c-runtime-versions '>=2.39' --plan --json > binding-change.json
```

Review the named file, typed operation, expected input SHA-256, patch and
validation consequences. The `plan_sha256` binds all these fields. Approve the
specific proposal by passing that digest separately:

```sh
sec platform c-bindings apply-plan binding-change.json \
    --approve REVIEWED_PLAN_SHA256 --json
```

Planning does not write the target or create a plan file; output redirection is
chosen by the caller. Without `--json`, planning prints the same proposal and
patch for human review. `sec help platform` includes both forms.

The service APIs are `PlanChange(file, operation)` and
`ApplyChange(plan, approvedSHA256)`. These call the existing `Apply` edit engine;
there is no second TOML parser, validation policy or formatter. Plans detach
operation pointers and use a versioned deterministic JSON representation.
Application rejects missing/different approval, modified or noncanonical plans,
changed/deleted inputs, or invalid edits. CLI JSON rejects unknown fields and
trailing documents. Approval cannot be supplied by adding a field to JSON.
A SHA-256 is an integrity identifier, not a secret credential: the trusted
caller remains responsible for obtaining authorization before calling Apply.

Application revalidates the typed change, then writes a temporary file in the
same directory and rechecks the input immediately before atomic replacement.
Existing permissions are retained. Cooperating in-process writers serialize
this last check and replacement. Arbitrary independent writers are not covered
by an operating-system compare-and-swap guarantee; filesystems supply atomic
replacement, not a portable hash-conditional rename. Failed preparation leaves
the target unchanged and removes its temporary file.

Reapplying an old changed plan is stale and requires a fresh review. Preparing
the same idempotent operation against the new input produces an unchanged plan;
applying that approved no-op does not rewrite the file.

These are structural C-binding edits. A plan does not establish target support
or validate selected module files: use `c-bindings validate --variant NAME` to
validate a selected environment and recompute resolution/fingerprints after a
changed commit. MCP transport, profile/provider edits and the complete harness acceptance
matrix remain separate governance work. Restricted project-root authorization
is supplied by the workspace facade below.

For restricted harness tools, use the [project-bound workspace facade](WORKSPACE.md)
rather than exposing the explicit-file CLI APIs directly. Its host-installed
policy supplies authorization and every filesystem operation is confined to the
selected project root and named C-binding file.
