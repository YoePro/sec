package sema

import (
	"strings"
	"testing"
)

// The ThreadLocal v2 surface is compiler-known so Sema and tooling observe
// one exact set of names and concrete generic substitutions.
//
// Rules:
//   - rules/concurrency/thread_local.md — §§4–5 "Exact declaration" and "No .Value property"
//   - rules/concurrency/thread_local.md — §§15–17 "Borrow, BorrowMut, and Replace"
func TestCompilerKnownThreadLocalV2Members(t *testing.T) {
	threadLocal := builtinTypes()["ThreadLocal"]
	threadLocal.TypeArgs = []Type{builtinTypes()["int"]}

	want := map[string]struct {
		id      string
		kind    TypeKind
		mutable bool
	}{
		"Borrow":    {id: "CKM-THREADLOCAL-BORROW", kind: ReferenceType},
		"BorrowMut": {id: "CKM-THREADLOCAL-BORROW-MUT", kind: ReferenceType, mutable: true},
		"Replace":   {id: "CKM-THREADLOCAL-REPLACE", kind: IntType},
	}
	for _, member := range CompilerKnownMembersForType(threadLocal, false) {
		expected, relevant := want[member.Name]
		if !relevant {
			continue
		}
		if member.ID != expected.id || member.Kind != CompilerKnownMethod || member.Result.Kind != expected.kind || member.Result.ReferenceMutable != expected.mutable {
			t.Fatalf("ThreadLocal[int].%s = %+v", member.Name, member)
		}
		if member.Result.Kind == ReferenceType && (member.Result.Element == nil || member.Result.Element.Kind != IntType) {
			t.Fatalf("ThreadLocal[int].%s result = %+v, want reference to int", member.Name, member.Result)
		}
		delete(want, member.Name)
	}
	if len(want) != 0 {
		t.Fatalf("missing ThreadLocal[int] members: %v", want)
	}
	for _, removed := range []string{"Value", "value", "Take", "Get", "Set"} {
		if _, exists := compilerKnownMember(threadLocal, removed, false); exists {
			t.Fatalf("removed ThreadLocal member %s remains visible", removed)
		}
	}
}

func TestThreadLocalV2CallsResolveExactTypes(t *testing.T) {
	input := `
module main

fn Access(local: ThreadLocal[int]) void {
    let shared: ref int := local.Borrow()
    let exclusive: ref mut int := local.BorrowMut()
    let previous: int := local.Replace(42)
}
`
	assertSemaErrors(t, analyzeSourceRaw(t, input), nil)
}

func TestThreadLocalReplaceValidatesArityAndPayload(t *testing.T) {
	input := `
module main

fn Invalid(local: ThreadLocal[int]) void {
    local.Borrow(1)
    local.Replace()
    local.Replace("wrong")
}
`
	errors := analyzeSourceRaw(t, input)
	if len(errors) != 3 {
		t.Fatalf("errors = %v, want 3", errors)
	}
	want := []string{
		"ThreadLocal[int].Borrow expects 0 arguments, got 1",
		"ThreadLocal[int].Replace expects 1 arguments, got 0",
		"ThreadLocal.Replace value must be int, got string",
	}
	for index, fragment := range want {
		if !strings.Contains(errors[index].Message, fragment) {
			t.Fatalf("error %d = %q, want %q", index, errors[index].Message, fragment)
		}
	}
}

func TestThreadLocalReplaceUsesOrdinaryOwnershipTransfer(t *testing.T) {
	valid := `
module main

@noCopy type Resource struct { value: int }

fn Replace(local: ThreadLocal[Resource], value: Resource) Resource {
    return local.Replace(<-value)
}
`
	assertSemaErrors(t, analyzeSourceRaw(t, valid), nil)

	invalid := `
module main

@noCopy type Resource struct { value: int }

fn Replace(local: ThreadLocal[Resource], value: Resource) Resource {
    return local.Replace(value)
}
`
	errors := analyzeSourceRaw(t, invalid)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "reusable source value passed to ThreadLocal.Replace must use explicit <- ownership transfer") {
		t.Fatalf("errors = %v", errors)
	}
}

func TestThreadLocalRemovedValuePropertyIsRejected(t *testing.T) {
	input := `
module main

fn Invalid(local: ThreadLocal[int]) int {
    return local.Value
}
`
	errors := analyzeSourceRaw(t, input)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "unknown member Value on ThreadLocal[int]") {
		t.Fatalf("errors = %v", errors)
	}
}
