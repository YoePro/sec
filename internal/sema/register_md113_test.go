package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
)

// MD-113: self.IsShadowKnown(T.Field...) is a compiler-known bool query with
// a static type-rooted field selector, legal only inside the owning impl on a
// hardware receiver; Read, Write and IsShadowKnown are reserved in register
// impl surfaces but stay available to ordinary types.
// Rules: rules/declarations/registers.md — § 11.7 (Amendment MD-113);
// rules/corrections/applied/MD-113-shadow-knowledge-across-calls-decision-20261030.md — §§ 3-4, 6, 9, 11, 12(1), 12(5).
func TestMD113ShadowKnownSelectorsAndReservation(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/register_shadow_known.sec")
	if err != nil {
		t.Fatal(err)
	}
	analyzer, errs := analyzeSourceWithAnalyzerRaw(t, string(data))
	wants := []struct {
		line    int
		message string
	}{
		{34, "IsShadowKnown needs the register-type-qualified field selector Control.Mode"},
		{35, "self is not a register type and the argument is not evaluated"},
		{36, "register Control has no field Unknown"},
		{37, "reserved register field _ cannot be selected"},
		{38, "not a runtime expression"},
		{39, "IsShadowKnown selector is rooted in Other, but the receiver is a Control hardware instance"},
		{40, "IsShadowKnown applies only to shadow fields; Ready is noshadow"},
		{41, "field Mode of register Control is not a nested register"},
		{46, "IsShadowKnown() is a hardware register operation; value is a software Control value"},
		{51, "IsShadowKnown is reserved in impl Other"},
		{54, "IsShadowKnown is reserved in impl Other"},
		{75, "IsShadowKnown() of hardware instance control is not public; it is reached only inside impl Control"},
	}
	if len(errs) != len(wants) {
		t.Fatalf("errors = %v, want %d", errs, len(wants))
	}
	for _, want := range wants {
		found := false
		for _, err := range errs {
			found = found || err.Line == want.line && strings.Contains(err.Message, want.message)
		}
		if !found {
			t.Errorf("missing %q at line %d in %v", want.message, want.line, errs)
		}
	}
	paths := map[string]string{}
	for call, operation := range analyzer.resolvedRegisterOperations {
		if operation.Kind != RegisterOperationShadowKnown {
			continue
		}
		if operation.ID != "CKM-REGISTER-IS-SHADOW-KNOWN" || operation.Register.Name != "Control" {
			t.Fatalf("operation = %+v", operation)
		}
		names := []string{}
		for _, field := range operation.FieldPath {
			names = append(names, field.Name)
		}
		paths[call.Arguments[0].(*ast.MemberExpression).String()] = strings.Join(names, ".")
	}
	for selector, path := range map[string]string{"Control.Mode": "Mode", "Control.IRQ.Pending": "IRQ.Pending", "Control.Command": "Command"} {
		if paths[selector] != path {
			t.Errorf("selector %s = %q, want %q (all %v)", selector, paths[selector], path, paths)
		}
	}
	for _, member := range CompilerKnownRegisterOperations(Type{Name: "Control"}) {
		if !isReservedRegisterOperationName(member.Name) || !strings.HasPrefix(member.ID, "CKM-REGISTER-") {
			t.Errorf("registry entry %+v", member)
		}
	}
}
