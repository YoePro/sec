package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/lexer"
)

// One receiver-resolved registry supplies identity, safety, signatures and
// effects to both the compiler and tooling, retaining nominal pointee names.
// Rules: rules/memory/raw_pointers.md — §§3(4),11–14,35(1–3),(6);
// rules/compiler/compiler_known_members.md — "RawPtr[T] compiler-known methods".
func TestRawPointerRegistrySurface(t *testing.T) {
	for _, element := range []Type{builtinType("byte"), builtinType("int"), builtinType("void"), {Name: "Reading", Kind: IntType, Named: true}} {
		pointer := builtinType("RawPtr")
		pointer.TypeArgs = []Type{element}
		seen := map[string]bool{}
		for _, member := range CompilerKnownMembersForType(pointer, false) {
			if !strings.HasPrefix(member.ID, "CKM-RAWPTR-") {
				continue
			}
			if seen[member.Name] {
				t.Fatalf("duplicate operation %s", member.Name)
			}
			seen[member.Name] = true
			if !member.Unsafe || !strings.HasPrefix(member.Signature, "unsafe fn ") || member.Kind != CompilerKnownMethod || member.Rule == "" || member.Receiver == "" || member.Documentation == "" || member.TargetRestriction == "" {
				t.Fatalf("incomplete shared metadata: %+v", member)
			}
			if strings.Contains(member.Signature, " T") || strings.Contains(member.Signature, "[T]") {
				t.Fatalf("unbound pointee: %+v", member)
			}
			switch member.Name {
			case "Read", "VolatileRead":
				if !sameConcreteType(member.Result, element) {
					t.Fatalf("lost pointee identity: %+v", member)
				}
			case "Offset", "AddBytes":
				if !sameConcreteType(member.Result, pointer) {
					t.Fatalf("lost pointer identity: %+v", member)
				}
			case "Write", "VolatileWrite":
				if member.Result.Kind != VoidType {
					t.Fatalf("write result: %+v", member)
				}
			case "Difference":
				if member.Result.Name != "int" {
					t.Fatalf("difference result: %+v", member)
				}
			default:
				t.Fatalf("unspecified operation: %+v", member)
			}
			wantEffects := 0
			if strings.HasPrefix(member.Name, "Volatile") {
				wantEffects = 1
			}
			if len(member.Effects) != wantEffects {
				t.Fatalf("ordinary/volatile effects conflated: %+v", member)
			}
			if !strings.Contains(CompilerKnownDocumentationForType(pointer, false), CompilerKnownSyntheticDefinition(member)) {
				t.Fatal("API documentation has diverged from the registry")
			}
		}
		want := 6
		if element.Name == "byte" {
			want = 7
		}
		if element.Kind == VoidType {
			want = 0
		}
		if len(seen) != want || seen["AddBytes"] != (element.Name == "byte") {
			t.Fatalf("%s surface: %v", typeDisplayName(pointer), seen)
		}
		for _, member := range CompilerKnownMembersForType(pointer, true) {
			if strings.HasPrefix(member.ID, "CKM-RAWPTR-") {
				t.Fatalf("instance operation became static: %+v", member)
			}
		}
	}
}

// Valid operation sites acquire exactly the metadata offered to completion;
// explicit inapplicable calls still get their operation-specific diagnostics.
// Rules: rules/memory/raw_pointers.md — §§3(4),11–14,34–35.
func TestRawPointerRegistryMatchesAnalyzedCalls(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/raw_pointer_registry_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
	assertSemaErrors(t, errors, nil)
	scanner := lexer.New(string(data))
	previous := lexer.Token{}
	calls := 0
	for token := scanner.NextToken(); token.Type != lexer.EOF; token = scanner.NextToken() {
		if previous.Type == lexer.DOT {
			member, found := analyzer.CompilerKnownMemberAt(token.File, token.Line, token.Column)
			if !found || !strings.HasPrefix(member.ID, "CKM-RAWPTR-") || !strings.HasPrefix(member.Signature, "unsafe fn ") {
				t.Fatalf("call %s lacks canonical registry metadata: %+v, %v", token.Lexeme, member, found)
			}
			calls++
		}
		previous = token
	}
	if calls != 25 {
		t.Fatalf("resolved call count = %d, want 25", calls)
	}
	invalid, err := os.ReadFile("../../testdata/tooling/raw_pointer_registry_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors = analyzeSourceRaw(t, string(invalid))
	if len(errors) != 7 {
		t.Fatalf("lost precise rejection paths: %v", errors)
	}
	for index, name := range []string{"Read", "Write", "VolatileRead", "VolatileWrite", "Offset", "Difference", "AddBytes"} {
		if !strings.Contains(errors[index].Message, "."+name) || strings.Contains(errors[index].Message, "unknown") {
			t.Fatalf("explicit invalid call lost operation diagnostic: %+v", errors[index])
		}
	}
}

// Borrowing a raw-pointer value does not turn it into a reference to its
// pointee; member lookup still addresses the underlying raw-pointer value.
// Rules: rules/foundations/grammar.md — "Member access";
// rules/memory/raw_pointers.md — §§4,35(6).
func TestRawPointerRegistryThroughReferences(t *testing.T) {
	pointer := builtinType("RawPtr")
	pointer.TypeArgs = []Type{builtinType("byte")}
	direct := CompilerKnownMembersForType(pointer, false)
	for _, mutable := range []bool{false, true} {
		reference := compilerKnownSharedReference(pointer)
		reference.ReferenceMutable = mutable
		count := 0
		for _, member := range CompilerKnownMembersForType(reference, false) {
			if !strings.HasPrefix(member.ID, "CKM-RAWPTR-") {
				continue
			}
			count++
			found := false
			for _, original := range direct {
				if original.ID == member.ID {
					found = true
					if original.Signature != member.Signature || original.Unsafe != member.Unsafe || !sameConcreteType(original.Result, member.Result) {
						t.Fatalf("reference changed raw-pointer contract: %+v, %+v", original, member)
					}
				}
			}
			if !found {
				t.Fatalf("reference invented a raw-pointer operation: %+v", member)
			}
		}
		if count != 7 {
			t.Fatalf("reference lost raw-pointer operations: %d", count)
		}
	}
}

// Static caller obligations accompany every unsafe raw operation. Parameter
// types and obligation text retain the concrete nominal pointee rather than
// implying that an unsafe context has validated the pointer.
// Rules: rules/memory/raw_pointers.md — §§5.2,11–15,35(2–6).
func TestRawPointerCallerContracts(t *testing.T) {
	pointer := builtinType("RawPtr")
	pointer.TypeArgs = []Type{{Name: "Reading", Kind: IntType, Named: true}}
	members := CompilerKnownMembersForType(pointer, false)
	count := 0
	for _, member := range members {
		if !strings.HasPrefix(member.ID, "CKM-RAWPTR-") {
			continue
		}
		count++
		if len(member.CallerObligations) == 0 {
			t.Fatalf("missing caller obligations: %+v", member)
		}
		wantParameters := 1
		if member.Name == "Read" || member.Name == "VolatileRead" {
			wantParameters = 0
		}
		if len(member.Parameters) != wantParameters {
			t.Fatalf("parameter count: %+v", member)
		}
		if wantParameters == 1 {
			parameter := member.Parameters[0]
			switch member.Name {
			case "Write", "VolatileWrite":
				if parameter.Name != "value" || !sameConcreteType(parameter.Type, pointer.TypeArgs[0]) {
					t.Fatalf("lost nominal write argument: %+v", parameter)
				}
			case "Offset":
				if parameter.Name != "count" || parameter.Type.Name != "int" {
					t.Fatalf("offset argument: %+v", parameter)
				}
			case "Difference":
				if parameter.Name != "other" || !sameConcreteType(parameter.Type, pointer) {
					t.Fatalf("difference argument: %+v", parameter)
				}
			}
		}
		for _, obligation := range member.CallerObligations {
			if strings.Contains(obligation, "`T`") || strings.Contains(obligation, "[T]") {
				t.Fatalf("unbound obligation: %s", obligation)
			}
			if !strings.Contains(CompilerKnownSyntheticDefinition(member), obligation) {
				t.Fatalf("API documentation lost caller obligation: %s", obligation)
			}
		}
		if member.Name == "Read" && !strings.Contains(strings.Join(member.CallerObligations, " "), "`Reading`") {
			t.Fatalf("lost nominal pointee in caller contract: %+v", member)
		}
		if strings.HasPrefix(member.Name, "Volatile") && !strings.Contains(strings.Join(member.CallerObligations, " "), "does not establish synchronization") {
			t.Fatalf("volatile obligations incomplete: %+v", member)
		}
	}
	if count != 6 {
		t.Fatalf("typed raw-pointer contracts: %d", count)
	}
	pointer.TypeArgs = []Type{builtinType("byte")}
	addBytes, found := compilerKnownMember(pointer, "AddBytes", false)
	if !found || len(addBytes.Parameters) != 1 || addBytes.Parameters[0].Type.Name != "int" || len(addBytes.CallerObligations) == 0 {
		t.Fatalf("byte arithmetic contract: %+v", addBytes)
	}
}
