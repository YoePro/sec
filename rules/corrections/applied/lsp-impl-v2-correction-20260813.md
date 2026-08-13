# Correction: LSP sync for `impl` revision 2.0

**Target:** `rules/tooling/lsp.md`  
**Source rule:** `rules/declarations/impl.md` revision 2.0  
**Date:** 2026-08-13

## Semantic tokens

Classify `new` as a language keyword.

Recognize contextual `init` and `free` lifecycle members inside `impl` without
misclassifying an ordinary function named `init` outside that form.

## Hover/signature help

For an initializer such as:

```sec
init(address: Address) ConnectError
```

show the error type as construction fallibility, not as a return type.

Recommended presentation:

```text
init(address: Address)
constructs: Connection
construction error: ConnectError
```

For:

```sec
new Connection(address)
```

hover/signature help should show:

- constructed type: `Connection`;
- selected `init` overload;
- construction error type when present;
- whether `try`/handling is required.

## Navigation

Go-to-definition/references should connect:

```sec
new Type(args...)
```

to the selected `init` declaration (or identify the permitted implicit
construction path).

For nested types and nested impls, navigation should use qualified identities
such as:

```text
Vehicle.Engine
```

## Diagnostics

Surface clear diagnostics for:

- explicit `self` parameters in canonical methods;
- duplicate init signatures across impl fragments;
- missing primary impl for `impl extends`;
- ordinary impl declared outside the target's defining module;
- `new` selecting a fallible initializer without `try`/handling;
- `new` used where no lifecycle construction path exists;
- construction resource with no provable cleanup path;
- direct calls to `free`.

## Completion

Inside an impl body, offer contextual lifecycle completions for `init` and
`free` where semantically valid.

At expression sites, completion/keyword support should include `new`.
