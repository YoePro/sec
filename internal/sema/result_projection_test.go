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
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "value result was consumed by Result.Ok() here and is no longer available") {
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
