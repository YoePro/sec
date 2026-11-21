# Rulebook corrections

This directory contains pending normative corrections to canonical Sec
rulebooks.

A correction is not an implementation tracker. Integrate its normative text into
the target rulebooks and give every one of its implementation points its own
item in the owning `governance/*.yaml` fragment. As soon as every point is there,
move the correction to `applied/` and append the application date to the basename
using `-YYYYMMDD.md` — whatever archival condition the correction itself or a
governance entry states (owner rule, 2026-11-21). Unfinished implementation never
keeps a correction in this directory; it stays tracked in governance.

Example:

```text
rules/corrections/example-correction.md
rules/corrections/applied/example-correction-20260813.md
```

The archived correction records what changed and which source authority caused
the change. Canonical language semantics remain in the target rulebook, and
mutable compiler progress remains in the owning `governance/*.yaml` fragment;
the root `implementation-status.yaml` is only a migration source.
