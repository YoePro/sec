# Dynamic-array membership

- Status: Applied normative correction
- Applied: 2026-09-09
- Authority: User request to support `value in values` for owning `T[]` arrays.
- Target: rules/foundations/operators.md

Extend collection membership from fixed arrays and slices to owning dynamic
arrays. Search the initialized logical length in source index order, evaluate
operands once, stop at the first equal element, and return false for empty
arrays. Membership itself does not allocate, consume elements, or mutate the
array. Array construction retains its own allocation and effect requirements.

Existing element equality and nominal type compatibility remain in force.
A table for a named `Status` receiver therefore uses `Status[]`.

Implementation and backend limitations are tracked in governance/sema.yaml.
