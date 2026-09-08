# Governance ledger fragments

`governance/` is the canonical, split implementation-status ledger.  It
replaces the large root ledger as the place for all new governance work.

## Read only what applies

1. Read this file and `index.yaml`.
2. Open the fragment whose `area` owns the requested behavior.
3. Read rulebooks and corrections named by that fragment's integrations.
4. Follow cross-links only when they affect the requested behavior.

Do not load every fragment merely to work in one area.

## Fragment contract

Every fragment uses the header:

```yaml
schema_version: 1
area: example-area
purpose: Short ownership description.
integrations: []
```

`integrations` uses the same entry shape and status vocabulary as the legacy
`implementation-status.yaml`: `implemented`, `partial`, and `planned`.
An integration has one canonical owner.  Related work is referenced through
rulebook paths or integration IDs; it is never copied into a second fragment.

## Migration from the legacy ledger

`../implementation-status.yaml` is a migration source, not a second canonical
ledger.  New entries belong here.  When moving an existing integration, copy
its complete entry into exactly one fragment, preserve its ID/history, remove
the root copy in the same change, and check that no duplicate ID remains.

Keep `index.yaml` small: it registers fragment paths and ownership only.  Add
or rename a fragment through the index in the same change.  Do not put full
integration entries or copied status text in the index.

## Choosing a fragment

Choose the primary semantic owner, not every affected compiler phase.  For
example, a raw-pointer integration that reaches Semantic IR and lowering still
lives in `memory.yaml`; a backend-only Sec MLIR package lives in
`lowering.yaml`.  Use `governance.yaml` only for governance/traceability work,
not as an overflow file.
