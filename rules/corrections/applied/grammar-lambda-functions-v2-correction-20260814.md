# Correction: lambda and callable grammar v2

- **Status:** Applied 2026-08-14
- **Created:** 2026-08-14
- **Last updated:** 2026-08-14
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Targets:** grammar and parser rulebooks

## Lambda expression

```text
LambdaExpression
    := CaptureClause? "fn" GenericLambdaForbidden "(" ParameterList? ")" Type Block
```

`GenericLambdaForbidden` means no generic parameter list is accepted after the anonymous `fn` in Sec 0.1.

The lambda expression itself is always introduced by plain `fn`.

`mut fn` and `-> fn` are callable types/contracts, not lambda-expression introducers.

## Capture clause

```text
CaptureClause
    := "capture" "(" CaptureEntryList? ")"

CaptureEntryList
    := CaptureEntry ("," CaptureEntry)* ","?

CaptureEntry
    := Identifier
     | "->" Identifier
     | "ref" Identifier
     | "ref" "mut" Identifier
```

Arbitrary expressions are not capture entries.

## Callable types

```text
CallableType
    := CallableCapability? "fn" "(" CallableParameterTypeList? ")" Type

CallableCapability
    := "mut"
     | "->"
```

Plain `fn` means shared/reusable callable capability.

Callable parameter types preserve function parameter modes:

```text
T
ref T
ref mut T
-> T
...T
```

Callable capability and consuming parameter mode are separate syntax positions.
