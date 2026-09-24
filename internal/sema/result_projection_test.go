package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// Rules:
//   - rules/errors/errorhandling.md — §6.1 and §29
//   - rules/corrections/applied/ownership-errorhandling-correction-20260824.md
func TestResultConsumingProjectionsReturnOptionsAndConsumeReceiver(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

type Failure enum error { Failed }

fn Success(result: Result[int, Failure]) Option[int] {
	return result.Ok()
}

fn Error(result: Result[int, Failure]) Option[Failure] {
	return result.Err()
}

fn Reuse(result: Result[int, Failure]) Result[int, Failure] {
	let value := result.Ok()
	discard value
	return result
}
`)
	if len(errors) != 1 ||
		!strings.Contains(errors[0].Message, "value result was consumed by Result.Ok() here and is no longer available") ||
		!strings.Contains(errors[0].Message, "use OkRef to inspect without consuming") ||
		errors[0].PreviousLine == 0 || errors[0].PreviousColumn == 0 {
		t.Fatalf("Result projection ownership errors = %v", errors)
	}

	resultType := Type{Name: "Result", Kind: ResultType, TypeArgs: []Type{builtinTypes()["int"], builtinTypes()["error"]}}
	want := map[string]TypeKind{"CKM-RESULT-OK": IntType, "CKM-RESULT-ERR": ErrorRootType}
	for _, member := range CompilerKnownMembersForType(resultType, false) {
		payloadKind, expected := want[member.ID]
		if !expected {
			continue
		}
		if member.Result.Name != "Option" || len(member.Result.TypeArgs) != 1 || member.Result.TypeArgs[0].Kind != payloadKind {
			t.Fatalf("Result projection member %s has result %+v", member.ID, member.Result)
		}
		delete(want, member.ID)
	}
	if len(want) != 0 {
		t.Fatalf("missing Result projection registry members: %v", want)
	}
}

func TestResultConsumingProjectionRejectsBorrowedReceiver(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

type Failure enum error { Failed }

fn Inspect(result: ref Result[int, Failure]) Option[int] {
	return result.Ok()
}
`)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "Ok() consumes an owned Result; use OkRef") {
		t.Fatalf("borrowed Result projection errors = %v", errors)
	}
}

func TestResultErrProjectionUseAfterConsumeSuggestsErrRef(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

type Failure enum error { Failed }

fn Reuse(result: Result[int, Failure]) Result[int, Failure] {
	let failure := result.Err()
	discard failure
	return result
}
`)
	if len(errors) != 1 ||
		!strings.Contains(errors[0].Message, "consumed by Result.Err()") ||
		!strings.Contains(errors[0].Message, "use ErrRef to inspect without consuming") ||
		errors[0].PreviousLine == 0 || errors[0].PreviousColumn == 0 {
		t.Fatalf("Result Err projection ownership errors = %v", errors)
	}
}

func TestResultConsumingProjectionRejectsNonDiscardableAlternatePayload(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

type Failure enum error { Failed }

fn Project(result: Result[Task[int], Failure]) Option[Failure] {
	return result.Err()
}
`)
	if len(errors) != 1 || errors[0].ID != diagnostics.NonDiscardableValue ||
		!strings.Contains(errors[0].Message, "alternate Task[int] payload") {
		t.Fatalf("non-discardable alternate Result payload errors = %v", errors)
	}
}

// Result projections are ordinary Option expressions after the compiler-known
// member has been resolved. Consequently, the narrow `if ... is None` syntax
// must keep working when its subject is a consuming projection rather than a
// previously bound Option local.
//
// Rules:
//   - rules/errors/errorhandling.md — §28 "if tests for Option"
//   - rules/errors/errorhandling.md — §29 "Result projection"
func TestResultConsumingProjectionsSupportOptionNoneIfTests(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

type Failure enum error { Failed }

fn HasFailure(result: Result[int, Failure]) bool {
	if result.Err() is not None {
		return true
	}
	return false
}

fn HasNoValue(result: Result[int, Failure]) bool {
	if result.Ok() is None {
		return true
	}
	return false
}
`)
	if len(errors) != 0 {
		t.Fatalf("Result projection Option None test errors = %v", errors)
	}
}

// Rules:
//   - rules/errors/errorhandling.md — §6.2 and §29
//   - rules/memory/borrowing.md — shared-borrow lifetime
func TestResultBorrowedProjectionsReturnReferenceOptionsWithoutConsuming(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

type Failure enum error { Failed }

fn Inspect(result: Result[int, Failure]) void {
	let success := result.OkRef
	let failure := result.ErrRef
	discard success
	discard failure
	discard result
}

fn InspectBorrowed(result: ref Result[int, Failure]) void {
	let success := result.OkRef
	let failure := result.ErrRef
	discard success
	discard failure
}
`)
	if len(errors) != 0 {
		t.Fatalf("borrowed Result projection errors = %v", errors)
	}

	resultType := Type{Name: "Result", Kind: ResultType, TypeArgs: []Type{builtinTypes()["int"], builtinTypes()["error"]}}
	want := map[string]TypeKind{"CKM-RESULT-OK-REF": IntType, "CKM-RESULT-ERR-REF": ErrorRootType}
	for _, member := range CompilerKnownMembersForType(resultType, false) {
		payloadKind, expected := want[member.ID]
		if !expected {
			continue
		}
		if member.Kind != CompilerKnownProperty || member.Result.Name != "Option" || len(member.Result.TypeArgs) != 1 {
			t.Fatalf("borrowed Result projection member %s has result %+v", member.ID, member.Result)
		}
		borrowed := member.Result.TypeArgs[0]
		if borrowed.Kind != ReferenceType || borrowed.ReferenceMutable || borrowed.Element == nil || borrowed.Element.Kind != payloadKind {
			t.Fatalf("borrowed Result projection member %s payload = %+v", member.ID, borrowed)
		}
		delete(want, member.ID)
	}
	if len(want) != 0 {
		t.Fatalf("missing borrowed Result projection registry members: %v", want)
	}
}

func TestResultBorrowedProjectionCannotEscapeLocalReceiver(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

type Failure enum error { Failed }

fn Escape() Option[ref int] {
	let result: Result[int, Failure] := Ok(42)
	return result.OkRef
}
`)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "cannot return value containing reference to local variable result") {
		t.Fatalf("borrowed Result projection escape errors = %v", errors)
	}
}

func TestResultBorrowedProjectionPreventsConflictingReceiverMutation(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

type Failure enum error { Failed }

fn Replace() void {
	let mut result: Result[int, Failure] := Ok(1)
	let view := result.OkRef
	result = Ok(2)
	discard view
	discard result
}
`)
	if len(errors) == 0 || !strings.Contains(errors[0].Message, "while it is shared borrowed") {
		t.Fatalf("borrowed Result projection mutation errors = %v", errors)
	}
}
