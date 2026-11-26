# Sec MCP adapter

`sec-mcp` is an optional local stdio adapter for Sec's project tooling,
including C bindings and MD-139 profile/allocation selections.
It uses the official [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk/tree/v1.8.0)
and the shared project-confined C-binding workspace service. It never shells out
to `sec` and adds no AI runtime or provider dependency.

## Build and launch

The adapter has its own Go module so ordinary compiler/LSP builds do not depend
on MCP. From the checkout root:

```sh
go -C cmd/sec-mcp build -trimpath -o ../../bin/sec-mcp .
go -C cmd/sec-mcp test -race ./...
bin/sec-mcp --project "$SEC_PROJECT_ROOT"
```

Go 1.25 or later is required. The explicitly selected project must have a valid
`.sec/sec.toml`; the server never discovers another root from cwd or parent
folders. Start it through an MCP client that communicates over stdin/stdout.
`--help` and diagnostics go to stderr; stdout contains only protocol messages.
The process exits when stdin closes. It opens no HTTP listener.

Example client configuration (replace the placeholders):

```json
{
  "mcpServers": {
    "sec": {
      "command": "<checkout>/bin/sec-mcp",
      "args": ["--project", "<project-root>"]
    }
  }
}
```

## Tools and approval

By default the server exposes these read-only tools:

| Tool | Purpose |
| --- | --- |
| `c-bindings.inspect` | Inspect the named C-binding configuration and input hash. |
| `c-bindings.validate` | Run the canonical structural validator. |
| `c-bindings.plan` | Produce a typed, comment-preserving change proposal. |
| `c-bindings.diff` | Show the intact proposal's patch. |
| `project.inspect` | Validate and inspect profile/allocation selections and scope provenance. |
| `project.resolve` | Resolve one Target–Variant profile and C-binding plan/fingerprint. |
| `project.plan` | Prepare a typed profile/allocation manifest proposal. |
| `project.diff` | Review the intact manifest proposal. |

Planning accepts one existing typed operation, for example:

```json
{
  "operation": {
    "kind": "environment.set",
    "environment": "linux-amd64-glibc",
    "c_runtime_versions": ">=2.39"
  }
}
```

The supported kinds are `package.set`, `environment.add`, `environment.set`,
`environment.remove`, `module.add` and `module.remove`. The underlying service
checks their existing schema and semantics. A plan response contains
`structuredContent.plan`, including the target project, named file, operation,
input/output hashes, patch, validation consequences and `plan_sha256`.

After the user/trusted caller has reviewed a specific proposal, the launcher
may restart the adapter with that exact project-bound digest:

```sh
bin/sec-mcp --project "$SEC_PROJECT_ROOT" --approve REVIEWED_PLAN_SHA256
```

`--approve` is repeatable. This exposes `c-bindings.apply` and `project.apply`, whose input is
`{"plan": <the intact plan object>}`. It accepts only exact preapproved hashes;
there is no wildcard or blanket write switch. Approval cannot come from model
prose, a request field or the plan itself. Digests identify content, not secret
credentials. The MCP client cannot replace startup policy or change the root.
Old changed plans fail after their input changes; prepare and review a new plan.

The services confine filesystem access to the selected root and the named
`.sec/platform/c-bindings.toml` or `.sec/sec.toml`, including during symlink replacement.
Resolution also reads the selected binding module sources through the same root. It reuses
the CLI's typed editor, validation and transaction engine. Tool hints truthfully
distinguish reads from writes, while the service enforces approval independently.
See [workspace safety](../../internal/platform/cbindings/WORKSPACE.md) and
[transactions](../../internal/platform/cbindings/TRANSACTIONS.md).

## Scope

`c-bindings.validate` performs structural validation. Use `project.resolve` with
an explicit Target and Variant, or the canonical CLI's
`platform c-bindings validate --variant NAME`, for selected binding resolution.
Neither tool supplies missing provider qualification or backend capabilities.
There are no shell, network, arbitrary-file or root-selection tools. Profile/provider
selection uses the canonical resolver; unknown providers cannot be activated. No harness TOML section or separate server configuration
file is introduced: this first adapter uses explicit launch arguments and a
local stdio lifetime only.

The executable name is `sec-mcp`; the reusable workspace service retains its own
identity. These are the initial implementation choices from harness notes §8.
Profile/provider configuration tooling and CLI/MCP acceptance are implemented.
Provider qualification and backend support retain their separate governance owners.

## Profile/allocation workflow

Use `project.plan` with an existing scope, for example:

```json
{"operation":{"target":"app","variant":"linux-amd64","profile":"hosted","mode":"noalloc"}}
```

Omit `target` to edit a reusable Variant, or omit `variant` to edit a Target.
Omitted fields preserve existing values; an empty string clears a local key and
restores inheritance. `provider` selects an existing verified catalog identity;
it never defines a provider. Current catalogs contain no verified entries, so
provider-backed activation is rejected, including `sec.system-heap.v1`.

The equivalent CLI uses the same service and emits deterministic JSON:

```sh
sec platform project inspect --project "$SEC_PROJECT_ROOT"
sec platform project resolve --project "$SEC_PROJECT_ROOT" --target app --variant linux-amd64
sec platform project plan --project "$SEC_PROJECT_ROOT" \
  --operation '{"target":"app","variant":"linux-amd64","mode":"noalloc"}' > plan-response.json
```

Save the response's `plan` object separately as `plan.json`, review it, then use
`sec platform project diff --project "$SEC_PROJECT_ROOT" --plan plan.json` and
the following command:

```sh
sec platform project apply-plan --project "$SEC_PROJECT_ROOT" \
  --plan plan.json --approve REVIEWED_PLAN_SHA256
```

`sec platform project --help` lists the syntax.
See [service contracts](../../internal/platform/projectconfig/README.md).

## Use with Codex

After building, register the local stdio command using explicit absolute paths:

```sh
codex mcp add sec-project -- "$SEC_CHECKOUT/bin/sec-mcp" --project "$SEC_PROJECT_ROOT"
codex mcp list
```

`SEC_CHECKOUT` and `SEC_PROJECT_ROOT` are example shell variables supplied by the
caller; the adapter itself never reads them as policy. In the IDE extension,
restart the extension after adding the MCP server. This registration starts
read-only; a client cannot approve a write through tool arguments. See the
[official MCP configuration documentation](https://developers.openai.com/codex/mcp).

Acceptance tests launch real stdio sessions and a temporary `-trimpath` compiler
binary to compare CLI/MCP plans, commits, validation diagnostics and canonical
binding/profile provenance. Run them with `go -C cmd/sec-mcp test -race ./...`;
this independent module is intentionally outside root `go test ./...`.
