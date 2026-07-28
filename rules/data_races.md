# Data Races

## Purpose

Data race analysis defines when concurrent memory access is invalid Sec.

This rulebook is planned and does not define final syntax.

It must cover:

- task/task races
- task/thread races
- thread/task races
- thread/thread races
- static mutable storage
- unsafe code limits

Using a physical thread does not weaken ownership, borrowing or synchronization
requirements.
