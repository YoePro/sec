# Blocking

## Purpose

Blocking defines operations that may suspend a task or block a physical thread.

This rulebook is planned and does not define final syntax.

It must distinguish:

- task suspension
- physical thread block or park
- blocking joins
- blocking channel operations
- `select` waits
- mutex waits

The source ownership and commit rules must remain the same across task and
thread contexts.
