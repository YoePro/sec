package sema

import (
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

func analyzeGenericImplSource(t *testing.T, source string) []string {
	t.Helper()
	p := parser.New(lexer.NewWithFile(source, "sample.sec"))
	parsed := p.Parse()
	messages := []string{}
	for _, diagnostic := range NewAnalyzer().Analyze(parsed.Program) {
		messages = append(messages, diagnostic.Message)
	}
	return messages
}

// A method of `impl Box[T]` called on a `Box[int]` receiver takes its signature
// from that instance: the receiver is exact, `T` is int, and the receiver
// capability rules still apply.
// Rules: rules/compiler/monomorphization.md §§ 8.2, 9.3; rules/declarations/functions.md §§ 22, 24.
func TestGenericImplMethodsUseReceiverArguments(t *testing.T) {
	const declarations = "module main\ntype Box[T] struct { Value: T, Count: int }\nimpl Box[T] {\n  fn Get() T { return self.Value }\n  fn Bump() void { self.Count += 1 }\n}\n"
	if errors := analyzeGenericImplSource(t, declarations+"fn Use() int {\n  let box := Box[int] { Value: 3, Count: 0 }\n  return box.Get()\n}\n"); len(errors) != 0 {
		t.Fatal(errors)
	}
	if errors := analyzeGenericImplSource(t, declarations+"fn Use() bool {\n  let mut box := Box[bool] { Value: false, Count: 0 }\n  box.Bump()\n  return box.Get()\n}\n"); len(errors) != 0 {
		t.Fatal(errors)
	}
	// Distinct instances keep distinct signatures.
	if errors := analyzeGenericImplSource(t, declarations+"fn Use() int {\n  let number := Box[int] { Value: 3, Count: 0 }\n  let flag := Box[bool] { Value: true, Count: 1 }\n  if flag.Get() {\n    return number.Get()\n  }\n  return 0\n}\n"); len(errors) != 0 {
		t.Fatal(errors)
	}
	errors := analyzeGenericImplSource(t, declarations+"fn Use() int {\n  let box := Box[int] { Value: 3, Count: 0 }\n  return box.Get() + box.Get()\n}\nfn Wrong() string {\n  let box := Box[int] { Value: 3, Count: 0 }\n  return box.Get()\n}\n")
	if len(errors) != 1 || !strings.Contains(errors[0], "must return string, got int") {
		t.Fatal(errors)
	}
	errors = analyzeGenericImplSource(t, declarations+"fn Use() void {\n  let box := Box[int] { Value: 3, Count: 0 }\n  box.Bump()\n}\n")
	if len(errors) != 1 || !strings.Contains(errors[0], "requires mutable receiver") {
		t.Fatal(errors)
	}
}
