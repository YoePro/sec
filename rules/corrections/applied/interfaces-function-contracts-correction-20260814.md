# Correction: functions and interface receiver contracts

- **Status:** Applied 2026-08-14
- **Created:** 2026-08-14
- **Last updated:** 2026-08-14
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Target:** `rules/declarations/interfaces.md`

Concrete methods continue to use ordinary `fn` with implicit `self`.

Interface bodyless method requirements use explicit receiver capability:

```sec
interface Resource {
    fn Status() Status
    mut fn Reset() void
    -> fn Detach() Handle
    static fn Parse(value: string) Result[Resource, ParseError]
}
```

The receiver modifier is independent from consuming ordinary parameters.

For example:

```sec
interface Sender {
    mut fn Send(-> message: Message) Result[void, SendError]
}
```

means:

- the receiver requires mutable/exclusive access;
- the `message` parameter is consumed.

Interface conformance must preserve both receiver capability and parameter ownership contracts.
