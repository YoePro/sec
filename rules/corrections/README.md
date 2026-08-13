# Rulebook corrections

This directory contains pending normative corrections to canonical Sec
rulebooks.

A correction remains in this directory until every required change has been
merged into its target rulebook. Once applied, move it to `applied/` and append
the application date to the basename using `-YYYYMMDD.md`.

Example:

```text
rules/corrections/example-correction.md
rules/corrections/applied/example-correction-20260813.md
```

The archived correction records what changed and which source authority caused
the change. Canonical language semantics remain in the target rulebook, and
mutable compiler progress remains in `implementation-status.yaml`.
