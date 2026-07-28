# Cancellation

## Purpose

Cancellation defines cooperative shutdown of tasks and threads.

This rulebook is planned and does not define final syntax.

It must preserve:

- cooperative task cancellation
- cooperative thread shutdown by default
- explicit unsafe target-specific hard termination only if later approved
- ownership-safe cleanup
- no silent destruction of live borrowed state
