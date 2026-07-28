# Deadlock Analysis

## Purpose

Deadlock analysis defines diagnostics for lock ordering and blocking waits.

This rulebook is planned and does not define final syntax.

It must cover:

- mutex guard lifetime
- blocking `join`
- `await`
- `select`
- nested locks
- task suspension while holding guards
- physical thread blocking while holding guards

The compiler may conservatively reject common unsafe patterns before this
rulebook is complete.
