package semantic

import "testing"

// A complete constant concat tree becomes one decoded string constant in
// Semantic IR. Its literal children never appear as intermediate operations.
//
// Rules:
//   - rules/foundations/operators.md — "Compile-time concatenation", "Maximal concatenation plan"
//   - rules/compiler/semantic_ir.md — §11 "Constants"
func TestCompileTimeStringConcatBuildsOneFoldedConstant(t *testing.T) {
	module, err := analyzedModule(t, `
module main

fn Folded() string {
    return "A" + '\u{03A9}' + "\nB"
}
`, 14)
	if err != nil {
		t.Fatal(err)
	}

	var constants []Operation
	for _, function := range module.Functions {
		if function.Name != "Folded" {
			continue
		}
		for _, block := range function.Blocks {
			for _, operation := range block.Operations {
				if operation.Kind == OpConstString {
					constants = append(constants, operation)
				}
			}
		}
	}
	if len(constants) != 1 || constants[0].String != "AΩ\nB" {
		t.Fatalf("string constants = %+v, want one folded decoded value", constants)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("Verify(): %v\n%s", err, Format(module))
	}
}
