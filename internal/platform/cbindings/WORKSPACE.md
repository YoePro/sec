# Project-bound harness safety

`OpenWorkspace(projectRoot, authorize)` is the restricted facade for a future
harness adapter. The trusted host selects the root explicitly once. The service
requires a valid `.sec/sec.toml` at that exact root and never discovers a parent
project or falls back to the current working directory.

The closed tool catalogue from `WorkspaceTools` is:

| Operation | Writes files | Trusted authorization |
| --- | --- | --- |
| `c-bindings.inspect` | No | No |
| `c-bindings.validate` | No | No |
| `c-bindings.plan` | No | No |
| `c-bindings.diff` | No | No |
| `c-bindings.apply` | Yes, after validation | Required |

Every tool is limited to `.sec/platform/c-bindings.toml` under the selected
project root. There is no path, shell command, URL or arbitrary file-write tool.
Inspection and validation are structural: they neither establish target support
nor activate providers. Unknown/unsupported target and module selectors retain
the owning C-binding validator's result.

The facade uses an `os.Root` directory handle for reads, temporary-file creation,
final input checks, removal and rename. Traversal and external symlink targets
are rejected by rooted filesystem operations even if links change after review.
The host's filesystem permissions remain authoritative. Hosts without the
required containment guarantee (currently JavaScript and Plan 9 hosts) reject
opening the service instead of using an unsafe fallback. This is a directory
containment boundary, not a process sandbox or a denial of mount/device access.

`WorkspaceChangePlan` wraps the ordinary `ChangePlan` with the selected project
identity and a digest covering both. `Apply` rejects a plan for another root or
file before consulting authorization. It then asks the `WorkspaceAuthorizer`
installed by trusted host code to authorize the exact `WorkspaceApprovalRequest`:
project root, named file, project-bound plan digest and typed operation kind.
A nil authorizer is read-only. No tool request or JSON field installs a policy,
provides authorization, changes the root or grants privileges.

The host must obtain approval through its user/trusted-caller channel and match
that specific request. It must not infer approval from model prose, merely from
receiving a plan or from a digest: digests identify content, not credentials.
A transport should map only the methods in this facade; the ordinary explicit-
file CLI APIs remain trusted-caller APIs and are not the restricted tool surface.

Planning, diff, validation and application reuse the existing typed editor and
transaction checks. They do not add a TOML parser, validation policy or alternate
file editor. [Transaction details](TRANSACTIONS.md) describe hashes, previews,
no-op behavior and atomic replacement. The same-directory algorithm is shared
between rooted and ordinary CLI adapters. Its final check serializes cooperating
in-process writers; it does not claim cross-process hash-conditional rename.
Closing the workspace makes later I/O fail without falling back to ordinary paths.

The stdio MCP adapter remains a separate governance item. No new executable,
server, harness configuration syntax, AI dependency or network requirement is
introduced here. Profile/provider mutations and complete harness acceptance
also retain their existing separate owners.
