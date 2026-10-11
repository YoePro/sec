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

`integrations` uses the entry shape and status vocabulary of the retired root
ledger: `implemented`, `partial`, and `planned`.
An integration has one canonical owner.  Related work is referenced through
rulebook paths or integration IDs; it is never copied into a second fragment.

## Known holes

A verified soundness hole (an invalid program the compiler currently
accepts) is recorded as a `remaining` item of the owning integration that
starts with `known hole (verified YYYY-MM-DD):`, names a minimal accepted
example and the violated rule.  List them with:

```text
grep -rn "known hole" governance/
```

Remove the item, and record the fix under `implemented`, when the hole is
closed.

## Legacy ledger

The root `implementation-status.yaml` was fully migrated into these fragments
and deleted on 2026-10-10.  Every integration, including the moved ones with
their IDs and history, now lives in exactly one fragment.  Historical
verification commands inside entries may still name the retired file.

Keep `index.yaml` small: it registers fragment paths and ownership only.  Add
or rename a fragment through the index in the same change.  Do not put full
integration entries or copied status text in the index.

## Choosing a fragment

Choose the primary semantic owner, not every affected compiler phase.  For
example, a raw-pointer integration that reaches Semantic IR and lowering still
lives in `memory.yaml`; a backend-only Sec MLIR package lives in
`lowering.yaml`.  Use `governance.yaml` only for governance/traceability work,
not as an overflow file.
